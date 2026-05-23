package protocol

import (
	"fmt"

	"github.com/dougsko/rfmpd/internal/protocol/pb"
	"google.golang.org/protobuf/proto"
)

// Encode serializes a Frame to protobuf binary for RF transmission.
func Encode(frame Frame) ([]byte, error) {
	pbFrame := &pb.Frame{}
	switch f := frame.(type) {
	case *MSG:
		pbFrame.Payload = &pb.Frame_Msg{Msg: msgToPB(f)}
	case *FRAG:
		pbFrame.Payload = &pb.Frame_Frag{Frag: fragToPB(f)}
	case *SVEC:
		pbFrame.Payload = &pb.Frame_Svec{Svec: svecToPB(f)}
	default:
		return nil, ErrUnknownFrameType
	}
	data, err := proto.Marshal(pbFrame)
	if err != nil {
		return nil, fmt.Errorf("proto marshal: %w", err)
	}
	return data, nil
}

// Decode deserializes protobuf binary received over RF into a Frame.
func Decode(data []byte) (Frame, error) {
	pbFrame := &pb.Frame{}
	if err := proto.Unmarshal(data, pbFrame); err != nil {
		return nil, ErrInvalidFrame
	}
	switch p := pbFrame.Payload.(type) {
	case *pb.Frame_Msg:
		return msgFromPB(p.Msg)
	case *pb.Frame_Frag:
		return fragFromPB(p.Frag)
	case *pb.Frame_Svec:
		return svecFromPB(p.Svec)
	case nil:
		return nil, ErrInvalidFrame
	}
	return nil, ErrUnknownFrameType
}

// EncodeMsgRaw serializes a MSG directly (without the Frame wrapper) for fragmentation.
func EncodeMsgRaw(msg *MSG) []byte {
	data, err := proto.Marshal(msgToPB(msg))
	if err != nil {
		return nil
	}
	return data
}

// DecodeMsgRaw deserializes raw MSG bytes reassembled from fragments.
func DecodeMsgRaw(data []byte) (*MSG, error) {
	pbMsg := &pb.Msg{}
	if err := proto.Unmarshal(data, pbMsg); err != nil {
		return nil, ErrInvalidFrame
	}
	return msgFromPB(pbMsg)
}

func msgToPB(m *MSG) *pb.Msg {
	pbMsg := &pb.Msg{
		Id:        m.ID[:],
		FromNode:  m.FromNode,
		Timestamp: ToEpoch(m.Time),
		Channel:   m.Channel,
		Body:      m.Body,
		Author:    m.Author,
	}
	if m.ReplyTo != nil {
		pbMsg.ReplyTo = m.ReplyTo[:]
	}
	if m.Seq != nil {
		pbMsg.Seq = uint32(*m.Seq)
	}
	return pbMsg
}

func msgFromPB(p *pb.Msg) (*MSG, error) {
	if len(p.Id) != 6 {
		return nil, ErrInvalidFrame
	}
	var id [6]byte
	copy(id[:], p.Id)

	msg := &MSG{
		ID:       id,
		FromNode: p.FromNode,
		Time:     FromEpoch(p.Timestamp),
		Channel:  p.Channel,
		Body:     p.Body,
		Author:   p.Author,
	}

	if len(p.ReplyTo) == 6 {
		var replyID [6]byte
		copy(replyID[:], p.ReplyTo)
		msg.ReplyTo = &replyID
	}

	if p.Seq > 0 {
		seq := int(p.Seq)
		msg.Seq = &seq
	}

	return msg, nil
}

func fragToPB(f *FRAG) *pb.Frag {
	return &pb.Frag{
		MsgId: f.MessageID[:],
		Idx:   uint32(f.Idx),
		Total: uint32(f.Total),
		Data:  f.Data,
	}
}

func fragFromPB(p *pb.Frag) (*FRAG, error) {
	if len(p.MsgId) != 6 {
		return nil, ErrInvalidFrame
	}
	var msgID [6]byte
	copy(msgID[:], p.MsgId)

	idx := int(p.Idx)
	total := int(p.Total)
	if total < 1 || total > MaxFragments || idx < 0 || idx >= total {
		return nil, ErrInvalidFrame
	}

	return &FRAG{
		MessageID: msgID,
		Idx:       idx,
		Total:     total,
		Data:      p.Data,
	}, nil
}

func svecToPB(s *SVEC) *pb.Svec {
	vec := make(map[string]uint32, len(s.Vector))
	for k, v := range s.Vector {
		vec[k] = uint32(v)
	}
	return &pb.Svec{
		FromNode: s.FromNode,
		Vector:   vec,
	}
}

func svecFromPB(p *pb.Svec) (*SVEC, error) {
	vec := make(map[string]int, len(p.Vector))
	for k, v := range p.Vector {
		vec[k] = int(v)
	}
	return &SVEC{
		FromNode: p.FromNode,
		Vector:   vec,
	}, nil
}
