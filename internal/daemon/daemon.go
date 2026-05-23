package daemon

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/dougsko/rfmpd/internal/config"
	"github.com/dougsko/rfmpd/internal/network"
	"github.com/dougsko/rfmpd/internal/protocol"
	"github.com/dougsko/rfmpd/internal/storage"
	gosync "github.com/dougsko/rfmpd/internal/sync"
)

type Daemon struct {
	Config     *config.Config
	ConfigPath string
	DB         *storage.Database
	Direwolf   *network.DirewolfClient
	Timing     *gosync.Timing
	Fragmenter *protocol.Fragmenter
	Logger     *slog.Logger

	startTime   time.Time
	apiServer   APIBroadcaster
}

type APIBroadcaster interface {
	BroadcastMessage(data map[string]interface{})
}

func New(cfg *config.Config, logger *slog.Logger) (*Daemon, error) {
	db, err := storage.Open(cfg.Storage.DatabasePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	direwolf := network.NewDirewolfClient(
		cfg.Network.DirewolfHost,
		cfg.Network.DirewolfPort,
		cfg.Node.Callsign,
		cfg.Node.SSID,
		time.Duration(cfg.Network.ReconnectInterval)*time.Second,
		logger,
	)

	timing := gosync.NewTiming(cfg.Timing.BaseDelay, cfg.Timing.Jitter)

	fragmenter := protocol.NewFragmenter(cfg.Protocol.FragmentThreshold)

	d := &Daemon{
		Config:     cfg,
		DB:         db,
		Direwolf:   direwolf,
		Timing:     timing,
		Fragmenter: fragmenter,
		Logger:     logger,
		startTime:  time.Now(),
	}

	direwolf.OnFrame = d.handleReceivedFrame
	return d, nil
}

func (d *Daemon) SetAPIServer(s APIBroadcaster) {
	d.apiServer = s
}

func (d *Daemon) Run(ctx context.Context) error {
	if !d.Config.Network.OfflineMode {
		go d.Direwolf.Run(ctx)
	}

	go d.transmissionLoop(ctx)
	go d.syncLoop(ctx)
	go d.cleanupLoop(ctx)

	<-ctx.Done()
	d.DB.Close()
	return nil
}

func (d *Daemon) handleReceivedFrame(info []byte) {
	frame, err := protocol.Decode(info)
	if err != nil {
		d.Logger.Debug("Failed to decode frame", "error", err)
		return
	}

	switch f := frame.(type) {
	case *protocol.MSG:
		d.handleMSG(f)
	case *protocol.FRAG:
		d.handleFRAG(f)
	case *protocol.SVEC:
		d.handleSVEC(f)
	}
}

func (d *Daemon) handleMSG(msg *protocol.MSG) {
	msgIDHex := protocol.IDToHex(msg.ID)

	isNew, err := d.DB.MarkSeenIfNew(msgIDHex, nil)
	if err != nil || !isNew {
		return
	}

	var replyToStr *string
	if msg.ReplyTo != nil {
		s := protocol.IDToHex(*msg.ReplyTo)
		replyToStr = &s
	}

	var authorVal interface{}
	if msg.Author != "" {
		authorVal = msg.Author
	}
	msgData := map[string]interface{}{
		"id":        msgIDHex,
		"from_node": msg.FromNode,
		"author":    authorVal,
		"timestamp": protocol.FormatTimestamp(msg.Time),
		"channel":   msg.Channel,
		"reply_to":  replyToStr,
		"body":      msg.Body,
		"seq":       msg.Seq,
		"raw_frame": "",
	}

	if _, err := d.DB.SaveMessage(msgData); err != nil {
		d.Logger.Error("Failed to save message", "id", msgIDHex, "error", err)
	}

	if d.apiServer != nil {
		clientMsg := d.serializeForClient(msgData)
		d.apiServer.BroadcastMessage(clientMsg)
	}

	if msg.FromNode == d.fullCallsign() {
		return
	}

	delay := d.Timing.CalculateRebroadcastDelay()
	frags := d.Fragmenter.FragmentMessage(msg)
	if len(frags) > 0 {
		for i, frag := range frags {
			fragData, err := json.Marshal(frag.ToDict())
			if err != nil {
				d.Logger.Error("Failed to marshal fragment", "error", err)
				continue
			}
			fragDelay := d.Timing.CalculateFragmentDelay(i, len(frags))
			if err := d.DB.QueueTransmission("FRAG", string(fragData), delay.Seconds()+fragDelay.Seconds()); err != nil {
				d.Logger.Error("Failed to queue fragment", "error", err)
			}
		}
	} else {
		frameData, err := json.Marshal(msg.ToDict())
		if err != nil {
			d.Logger.Error("Failed to marshal message", "error", err)
			return
		}
		if err := d.DB.QueueTransmission("MSG", string(frameData), delay.Seconds()); err != nil {
			d.Logger.Error("Failed to queue message", "error", err)
		}
	}
	if err := d.DB.MarkSeen(msgIDHex, nil, true); err != nil {
		d.Logger.Error("Failed to mark seen", "id", msgIDHex, "error", err)
	}
	d.DB.IncrementRebroadcastCount(msgIDHex)
}

func (d *Daemon) handleFRAG(frag *protocol.FRAG) {
	msgIDHex := protocol.IDToHex(frag.MessageID)
	idx := frag.Idx
	isNew, err := d.DB.MarkSeenIfNew(msgIDHex, &idx)
	if err != nil || !isNew {
		return
	}

	d.DB.SaveFragment(msgIDHex, frag.Idx, frag.Total, frag.Data)

	_, reassembled := d.Fragmenter.AddFragment(frag)
	if reassembled != nil {
		d.handleMSG(reassembled)
	}
}

func (d *Daemon) handleSVEC(svec *protocol.SVEC) {
	d.DB.MarkSeen(svec.FromNode, nil, false)
	d.DB.UpdateNodeSync(svec.FromNode)

	ourClock, err := d.DB.GetVectorClock()
	if err != nil {
		return
	}

	// Build a merged set of all nodes to check
	nodesToCheck := make(map[string]int)
	for node, remoteSeq := range svec.Vector {
		nodesToCheck[node] = remoteSeq
	}
	// Add nodes we know about but the remote doesn't (implicit seq 0)
	for node := range ourClock {
		if _, ok := nodesToCheck[node]; !ok {
			nodesToCheck[node] = 0
		}
	}

	for node, remoteSeq := range nodesToCheck {
		if node == svec.FromNode {
			continue
		}

		msgs, err := d.DB.GetMessagesAfterSeq(node, remoteSeq)
		if err != nil || len(msgs) == 0 {
			continue
		}
		for _, msg := range msgs {
			msgFrame := d.messageRowToFrame(msg)
			if msgFrame == nil {
				continue
			}

			delay := d.Timing.CalculateRebroadcastDelay()
			frags := d.Fragmenter.FragmentMessage(msgFrame)
			if len(frags) > 0 {
				for i, frag := range frags {
					fragData, err := json.Marshal(frag.ToDict())
					if err != nil {
						d.Logger.Error("failed to marshal fragment", "err", err)
						continue
					}
					fragDelay := d.Timing.CalculateFragmentDelay(i, len(frags))
					d.DB.QueueTransmission("FRAG", string(fragData), delay.Seconds()+fragDelay.Seconds())
				}
			} else {
				frameData, err := json.Marshal(msgFrame.ToDict())
				if err != nil {
					d.Logger.Error("failed to marshal message frame", "err", err)
					continue
				}
				d.DB.QueueTransmission("MSG", string(frameData), delay.Seconds())
			}
		}
	}
}

func (d *Daemon) transmissionLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		item, err := d.DB.GetNextTransmission()
		if err != nil {
			time.Sleep(1 * time.Second)
			continue
		}

		if item == nil {
			time.Sleep(100 * time.Millisecond)
			continue
		}

		id := item["id"].(int64)
		frameType := item["frame_type"].(string)
		frameData := item["frame_data"].(string)

		var frameDict map[string]string
		if err := json.Unmarshal([]byte(frameData), &frameDict); err != nil {
			d.DB.MarkTransmitted(id)
			continue
		}

		var frame protocol.Frame
		switch frameType {
		case "MSG":
			frame, _ = protocol.MSGFromDict(frameDict)
		case "FRAG":
			frame, _ = protocol.FRAGFromDict(frameDict)
		case "SVEC":
			frame, _ = protocol.SVECFromDict(frameDict)
		default:
			d.DB.MarkTransmitted(id)
			continue
		}

		if frame == nil {
			d.DB.MarkTransmitted(id)
			continue
		}

		encoded, err := protocol.Encode(frame)
		if err != nil {
			d.Logger.Error("Failed to encode frame", "type", frameType, "error", err)
			d.DB.MarkTransmitted(id)
			continue
		}
		if err := d.Direwolf.SendFrame(encoded); err != nil {
			d.DB.MarkTransmissionFailed(id, 3)
			time.Sleep(1 * time.Second)
			continue
		}

		d.DB.MarkTransmitted(id)
		d.Timing.RecordTransmission()

		// Mark the message as transmitted if this was a MSG frame
		if frameType == "MSG" {
			if msgID, ok := frameDict["id"]; ok {
				d.DB.MarkMessageTransmitted(msgID)
			}
		}
	}
}

func (d *Daemon) syncLoop(ctx context.Context) {
	interval := time.Duration(d.Config.Sync.SyncInterval) * time.Second
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.broadcastSVEC()
		}
	}
}

func (d *Daemon) broadcastSVEC() {
	clock, err := d.DB.GetVectorClock()
	if err != nil {
		return
	}

	svec := &protocol.SVEC{
		FromNode: d.fullCallsign(),
		Vector:   clock,
	}

	frameData, err2 := json.Marshal(svec.ToDict())
	if err2 != nil {
		d.Logger.Error("failed to marshal SVEC", "err", err2)
		return
	}
	delay := d.Timing.CalculateSyncDelay()
	d.DB.QueueTransmission("SVEC", string(frameData), delay.Seconds())
}

func (d *Daemon) cleanupLoop(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			d.DB.CleanupSeenCache(3600)
			d.DB.CleanupTransmissionQueue()
			d.DB.CleanupOldFragments(3600)
			d.Fragmenter.CleanupExpired()
		}
	}
}

func (d *Daemon) fullCallsign() string {
	return d.Config.FullCallsign()
}

// API interface methods

func (d *Daemon) SendMessage(channel, body string, replyTo *string, author *string) (map[string]interface{}, error) {
	fromNode := d.fullCallsign()

	senderForID := fromNode
	if author != nil && *author != "" {
		senderForID = *author
	}

	now := time.Now()
	epoch := protocol.ToEpoch(now)
	msgID := protocol.GenerateMessageID(senderForID, epoch, body)
	msgIDHex := protocol.IDToHex(msgID)
	ts := protocol.FormatTimestamp(now)

	msgData := map[string]interface{}{
		"id":        msgIDHex,
		"from_node": fromNode,
		"author":    author,
		"timestamp": ts,
		"channel":   channel,
		"reply_to":  replyTo,
		"body":      body,
		"raw_frame": "",
	}

	_, seq, err := d.DB.SaveMessageWithSeq(msgData, fromNode)
	if err != nil {
		return nil, err
	}

	var authorStr string
	if author != nil {
		authorStr = *author
	}
	msg := &protocol.MSG{
		ID:       msgID,
		FromNode: fromNode,
		Time:     now.UTC(),
		Channel:  channel,
		Body:     body,
		Seq:      &seq,
		Author:   authorStr,
	}
	if replyTo != nil {
		replyID, err := protocol.IDFromHex(*replyTo)
		if err == nil {
			msg.ReplyTo = &replyID
		}
	}

	frags := d.Fragmenter.FragmentMessage(msg)
	if len(frags) > 0 {
		for i, frag := range frags {
			frameData, _ := json.Marshal(frag.ToDict())
			delay := d.Timing.CalculateFragmentDelay(i, len(frags))
			d.DB.QueueTransmission("FRAG", string(frameData), delay.Seconds())
		}
	} else {
		frameData, _ := json.Marshal(msg.ToDict())
		delay := d.Timing.CalculateDelay()
		d.DB.QueueTransmission("MSG", string(frameData), delay.Seconds())
	}

	d.DB.MarkSeen(msgIDHex, nil, false)

	if author != nil && *author != "" {
		d.DB.UpdateUserStats(*author)
	}

	result := d.serializeForClient(msgData)

	if d.apiServer != nil {
		d.apiServer.BroadcastMessage(result)
	}

	return result, nil
}

func (d *Daemon) GetStats() (map[string]interface{}, error) {
	uptime := time.Since(d.startTime).Seconds()
	msgCount, _ := d.DB.GetMessageCount()
	activeNodes, _ := d.DB.GetActiveNodes(3600)
	vectorClock, _ := d.DB.GetVectorClock()

	return map[string]interface{}{
		"uptime_seconds": uptime,
		"message_count":  msgCount,
		"active_nodes":   len(activeNodes),
		"vector_clock":   vectorClock,
		"timing":         d.Timing.GetStats(),
	}, nil
}

func (d *Daemon) GetConfig() (string, int, string) {
	return d.Config.Node.Callsign, d.Config.Node.SSID, d.fullCallsign()
}

func (d *Daemon) SetCallsign(callsign string, ssid int) error {
	d.Config.Node.Callsign = strings.ToUpper(callsign)
	d.Config.Node.SSID = ssid
	d.Direwolf.Callsign = d.Config.Node.Callsign
	d.Direwolf.SSID = ssid

	savePath := d.ConfigPath
	if savePath == "" {
		savePath = d.Config.LoadedFrom
	}
	if savePath != "" {
		if err := d.Config.SaveToFile(savePath); err != nil {
			d.Logger.Warn("Failed to persist config change", "error", err)
		}
	}
	return nil
}

func (d *Daemon) GetFullConfig() *config.Config {
	return d.Config
}

func (d *Daemon) SaveConfig(cfg *config.Config) error {
	path := d.ConfigPath
	if path == "" {
		path = d.Config.LoadedFrom
	}
	if path == "" {
		return fmt.Errorf("no config file path known")
	}
	cfg.LoadedFrom = path
	return cfg.SaveToFile(path)
}

func (d *Daemon) IsConnected() bool {
	return d.Direwolf.IsConnected()
}

func (d *Daemon) GetDB() interface{} {
	return d.DB
}

func (d *Daemon) messageRowToFrame(msg *storage.MessageRow) *protocol.MSG {
	id, err := protocol.IDFromHex(msg.ID)
	if err != nil {
		return nil
	}
	ts, err := protocol.ParseTimestamp(msg.Timestamp)
	if err != nil {
		return nil
	}
	var authorStr string
	if msg.Author != nil {
		authorStr = *msg.Author
	}
	frame := &protocol.MSG{
		ID:       id,
		FromNode: msg.FromNode,
		Time:     ts,
		Channel:  msg.Channel,
		Body:     msg.Body,
		Seq:      msg.Seq,
		Author:   authorStr,
	}
	if msg.ReplyTo != nil {
		replyID, err := protocol.IDFromHex(*msg.ReplyTo)
		if err == nil {
			frame.ReplyTo = &replyID
		}
	}
	return frame
}

func (d *Daemon) serializeForClient(msg map[string]interface{}) map[string]interface{} {
	result := map[string]interface{}{
		"id":        msg["id"],
		"from_node": msg["from_node"],
		"author":    msg["author"],
		"timestamp": msg["timestamp"],
		"channel":   msg["channel"],
		"reply_to":  msg["reply_to"],
		"body":      msg["body"],
	}

	if ra, ok := msg["received_at"]; ok {
		switch v := ra.(type) {
		case int64:
			result["received_at"] = time.Unix(v, 0).Format(time.RFC3339)
		default:
			result["received_at"] = time.Now().Format(time.RFC3339)
		}
	} else {
		result["received_at"] = time.Now().Format(time.RFC3339)
	}
	result["transmitted_at"] = nil
	return result
}
