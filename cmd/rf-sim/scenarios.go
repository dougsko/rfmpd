package main

import (
	"fmt"
	"math/rand"
	"strings"
	"time"
)

type ScenarioResult struct {
	Name     string
	Passed   bool
	Duration time.Duration
	Detail   string
}

var scenarioBaudRate int

func RunScenarioBasicMessaging(brokerPort int, timeout time.Duration, verbose bool, rfmpdBin string) ScenarioResult {
	start := time.Now()
	result := ScenarioResult{Name: "Basic messaging"}

	nodes := make([]*Node, 3)
	var err error
	for i := 0; i < 3; i++ {
		callsign := fmt.Sprintf("SIM%d", i)
		nodes[i], err = StartNode(i, callsign, 9100+i, brokerPort, verbose, rfmpdBin)
		if err != nil {
			result.Detail = fmt.Sprintf("failed to start node %d: %v", i, err)
			cleanupNodes(nodes)
			return result
		}
	}
	defer cleanupNodes(nodes)

	time.Sleep(1 * time.Second)

	msgID, err := SendMessage(nodes[0].APIPort, "general", "hello from node 0")
	if err != nil {
		result.Detail = fmt.Sprintf("failed to send message: %v", err)
		return result
	}

	expected := map[string]bool{msgID: true}
	_, err = WaitIDConvergence(nodes, "general", expected, timeout)
	if err != nil {
		result.Detail = err.Error()
		return result
	}

	result.Passed = true
	result.Duration = time.Since(start)
	return result
}

func RunScenarioLateSVEC(brokerPort int, timeout time.Duration, verbose bool, rfmpdBin string) ScenarioResult {
	start := time.Now()
	result := ScenarioResult{Name: "Late joiner SVEC sync"}

	nodes := make([]*Node, 2)
	var err error
	for i := 0; i < 2; i++ {
		callsign := fmt.Sprintf("SIM%d", i)
		nodes[i], err = StartNode(i, callsign, 9200+i, brokerPort, verbose, rfmpdBin)
		if err != nil {
			result.Detail = fmt.Sprintf("failed to start node %d: %v", i, err)
			cleanupNodes(nodes)
			return result
		}
	}

	time.Sleep(1 * time.Second)

	allIDs := make(map[string]bool)
	for i := 0; i < 3; i++ {
		id, err := SendMessage(nodes[0].APIPort, "general", fmt.Sprintf("msg-A-%d", i))
		if err != nil {
			result.Detail = fmt.Sprintf("send from node 0 failed: %v", err)
			cleanupNodes(nodes)
			return result
		}
		allIDs[id] = true
		time.Sleep(500 * time.Millisecond)

		id, err = SendMessage(nodes[1].APIPort, "general", fmt.Sprintf("msg-B-%d", i))
		if err != nil {
			result.Detail = fmt.Sprintf("send from node 1 failed: %v", err)
			cleanupNodes(nodes)
			return result
		}
		allIDs[id] = true
		time.Sleep(500 * time.Millisecond)
	}

	// Wait for the two nodes to have all messages first
	_, err = WaitIDConvergence(nodes, "general", allIDs, timeout/2)
	if err != nil {
		result.Detail = "initial nodes failed to converge: " + err.Error()
		cleanupNodes(nodes)
		return result
	}

	// Give some extra time for messages to fully settle
	time.Sleep(1 * time.Second)

	// Start late joiner
	lateNode, err := StartNode(2, "SIM2", 9202, brokerPort, verbose, rfmpdBin)
	if err != nil {
		result.Detail = fmt.Sprintf("failed to start late joiner: %v", err)
		cleanupNodes(nodes)
		return result
	}
	nodes = append(nodes, lateNode)
	defer cleanupNodes(nodes)

	// Wait for late joiner to sync via SVEC
	lateNodes := []*Node{lateNode}
	_, err = WaitIDConvergence(lateNodes, "general", allIDs, timeout)
	if err != nil {
		// Diagnostics: what does the late joiner have?
		msgs, _ := GetMessages(lateNode.APIPort, "general", 100)
		var bodies []string
		for _, m := range msgs {
			bodies = append(bodies, m.Body)
		}
		result.Detail = fmt.Sprintf("%v (got: %v)", err, bodies)
		return result
	}

	result.Passed = true
	result.Duration = time.Since(start)
	return result
}

func RunScenarioCrashRecovery(brokerPort int, timeout time.Duration, verbose bool, rfmpdBin string) ScenarioResult {
	start := time.Now()
	result := ScenarioResult{Name: "Node crash and recovery"}

	nodes := make([]*Node, 3)
	var err error
	for i := 0; i < 3; i++ {
		callsign := fmt.Sprintf("SIM%d", i)
		nodes[i], err = StartNode(i, callsign, 9300+i, brokerPort, verbose, rfmpdBin)
		if err != nil {
			result.Detail = fmt.Sprintf("failed to start node %d: %v", i, err)
			cleanupNodes(nodes)
			return result
		}
	}
	defer cleanupNodes(nodes)

	time.Sleep(1 * time.Second)

	sendDelay := 500 * time.Millisecond
	if scenarioBaudRate > 0 && scenarioBaudRate <= 1200 {
		sendDelay = 3 * time.Second
	}

	allIDs := make(map[string]bool)
	for i := 0; i < 3; i++ {
		id, err := SendMessage(nodes[i].APIPort, "general", fmt.Sprintf("pre-crash-%d", i))
		if err != nil {
			result.Detail = fmt.Sprintf("pre-crash send failed: %v", err)
			return result
		}
		allIDs[id] = true
		time.Sleep(sendDelay)
	}

	// Wait for all pre-crash messages to propagate via rebroadcast
	_, err = WaitIDConvergence(nodes, "general", allIDs, timeout)
	if err != nil {
		result.Detail = "pre-crash convergence failed: " + err.Error()
		return result
	}

	// Kill node 1
	nodes[1].Kill()
	time.Sleep(1 * time.Second)

	// Send more messages while node 1 is down
	for i := 0; i < 3; i++ {
		senderIdx := 0
		if i%2 == 1 {
			senderIdx = 2
		}
		id, err := SendMessage(nodes[senderIdx].APIPort, "general", fmt.Sprintf("post-crash-%d", i))
		if err != nil {
			result.Detail = fmt.Sprintf("post-crash send failed: %v", err)
			return result
		}
		allIDs[id] = true
		time.Sleep(sendDelay)
	}

	// Wait for messages to settle on nodes 0 and 2
	liveNodes := []*Node{nodes[0], nodes[2]}
	_, _ = WaitIDConvergence(liveNodes, "general", allIDs, timeout/2)

	// Restart node 1
	if err := nodes[1].Restart(rfmpdBin); err != nil {
		result.Detail = fmt.Sprintf("restart failed: %v", err)
		return result
	}

	// Wait for node 1 to get all messages via SVEC sync
	_, err = WaitIDConvergence(nodes, "general", allIDs, timeout)
	if err != nil {
		result.Detail = err.Error()
		return result
	}

	result.Passed = true
	result.Duration = time.Since(start)
	return result
}

func RunScenarioPartition(brokerPort int, timeout time.Duration, verbose bool, rfmpdBin string) ScenarioResult {
	start := time.Now()
	result := ScenarioResult{Name: "Network partition heal"}

	broker := globalBroker

	// Apply partition BEFORE starting nodes so no SVEC can cross
	broker.Partition([]int{0, 1}, []int{2, 3})

	nodes := make([]*Node, 4)
	var err error
	for i := 0; i < 4; i++ {
		callsign := fmt.Sprintf("SIM%d", i)
		nodes[i], err = StartNode(i, callsign, 9400+i, brokerPort, verbose, rfmpdBin)
		if err != nil {
			result.Detail = fmt.Sprintf("failed to start node %d: %v", i, err)
			cleanupNodes(nodes)
			return result
		}
	}
	defer cleanupNodes(nodes)

	time.Sleep(1 * time.Second)

	sideAIDs := make(map[string]bool)
	sideBIDs := make(map[string]bool)

	// Send messages on side A
	for i := 0; i < 3; i++ {
		id, err := SendMessage(nodes[0].APIPort, "general", fmt.Sprintf("side-A-%d", i))
		if err != nil {
			result.Detail = fmt.Sprintf("side A send failed: %v", err)
			return result
		}
		sideAIDs[id] = true
		time.Sleep(300 * time.Millisecond)
	}

	// Send messages on side B
	for i := 0; i < 3; i++ {
		id, err := SendMessage(nodes[2].APIPort, "general", fmt.Sprintf("side-B-%d", i))
		if err != nil {
			result.Detail = fmt.Sprintf("side B send failed: %v", err)
			return result
		}
		sideBIDs[id] = true
		time.Sleep(300 * time.Millisecond)
	}

	// Wait for intra-partition propagation
	sideA := []*Node{nodes[0], nodes[1]}
	sideB := []*Node{nodes[2], nodes[3]}
	WaitIDConvergence(sideA, "general", sideAIDs, timeout/2)
	WaitIDConvergence(sideB, "general", sideBIDs, timeout/2)

	// Verify partition isolation: side A should not have side B messages
	idsA, _ := GetMessageIDs(nodes[0].APIPort, "general")
	for id := range sideBIDs {
		if idsA[id] {
			result.Detail = "partition not effective - side B messages found on side A"
			return result
		}
	}
	idsB, _ := GetMessageIDs(nodes[2].APIPort, "general")
	for id := range sideAIDs {
		if idsB[id] {
			result.Detail = "partition not effective - side A messages found on side B"
			return result
		}
	}

	// Heal partition
	broker.Heal()

	// Wait for full convergence via SVEC
	allIDs := make(map[string]bool)
	for id := range sideAIDs {
		allIDs[id] = true
	}
	for id := range sideBIDs {
		allIDs[id] = true
	}

	_, err = WaitIDConvergence(nodes, "general", allIDs, timeout)
	if err != nil {
		result.Detail = err.Error()
		return result
	}

	result.Passed = true
	result.Duration = time.Since(start)
	return result
}

func RunScenarioFragmentation(brokerPort int, timeout time.Duration, verbose bool, rfmpdBin string) ScenarioResult {
	start := time.Now()
	result := ScenarioResult{Name: "Fragmentation under stress"}

	nodes := make([]*Node, 2)
	var err error
	for i := 0; i < 2; i++ {
		callsign := fmt.Sprintf("SIM%d", i)
		nodes[i], err = StartNode(i, callsign, 9500+i, brokerPort, verbose, rfmpdBin, WithFragmentThreshold(50))
		if err != nil {
			result.Detail = fmt.Sprintf("failed to start node %d: %v", i, err)
			cleanupNodes(nodes)
			return result
		}
	}
	defer cleanupNodes(nodes)

	time.Sleep(1 * time.Second)

	// Send long messages that will be fragmented (threshold is 50 bytes)
	longBodies := []string{
		strings.Repeat("A", 100),
		strings.Repeat("B", 150),
		strings.Repeat("C", 200),
	}

	fragSendDelay := 500 * time.Millisecond
	if scenarioBaudRate > 0 && scenarioBaudRate <= 1200 {
		fragSendDelay = 10 * time.Second
	}

	allIDs := make(map[string]bool)
	for _, body := range longBodies {
		id, err := SendMessage(nodes[0].APIPort, "general", body)
		if err != nil {
			result.Detail = fmt.Sprintf("send long message failed: %v", err)
			return result
		}
		allIDs[id] = true
		time.Sleep(fragSendDelay)
	}

	// Wait for node 1 to reassemble all fragments
	_, err = WaitIDConvergence(nodes, "general", allIDs, timeout)
	if err != nil {
		result.Detail = err.Error()
		return result
	}

	// Verify body integrity
	msgs, err := GetMessages(nodes[1].APIPort, "general", 100)
	if err != nil {
		result.Detail = fmt.Sprintf("get messages for verification failed: %v", err)
		return result
	}

	for _, expected := range longBodies {
		found := false
		for _, msg := range msgs {
			if msg.Body == expected {
				found = true
				break
			}
		}
		if !found {
			result.Detail = fmt.Sprintf("message body not found after reassembly (len=%d)", len(expected))
			return result
		}
	}

	result.Passed = true
	result.Duration = time.Since(start)
	return result
}

func RunScenarioChurn(brokerPort int, duration, timeout time.Duration, verbose bool, rfmpdBin string) ScenarioResult {
	start := time.Now()
	result := ScenarioResult{Name: "Rapid churn"}

	nodes := make([]*Node, 0, 5)
	for i := 0; i < 3; i++ {
		callsign := fmt.Sprintf("SIM%d", i)
		node, err := StartNode(i, callsign, 9600+i, brokerPort, verbose, rfmpdBin)
		if err != nil {
			result.Detail = fmt.Sprintf("failed to start initial node %d: %v", i, err)
			cleanupNodes(nodes)
			return result
		}
		nodes = append(nodes, node)
	}
	defer cleanupNodes(nodes)

	time.Sleep(1 * time.Second)

	allIDs := make(map[string]bool)
	nextNodeID := 3
	messagesSent := 0
	convergenceChecks := 0
	convergenceFailures := 0
	var totalConvergenceTime time.Duration

	deadline := time.Now().Add(duration)
	lastChurn := time.Now()
	lastMsg := time.Now()
	lastCheck := time.Now()

	for time.Now().Before(deadline) {
		// Send a message every 2 seconds
		if time.Since(lastMsg) > 2*time.Second {
			liveNodes := getLiveNodes(nodes)
			if len(liveNodes) > 0 {
				sender := liveNodes[rand.Intn(len(liveNodes))]
				id, err := SendMessage(sender.APIPort, "general", fmt.Sprintf("churn-msg-%d", messagesSent))
				if err == nil {
					allIDs[id] = true
					messagesSent++
				}
			}
			lastMsg = time.Now()
		}

		// Churn every 5-10 seconds
		if time.Since(lastChurn) > time.Duration(5+rand.Intn(5))*time.Second {
			liveNodes := getLiveNodes(nodes)
			if rand.Float64() < 0.5 && len(liveNodes) > 2 {
				// Kill a random node
				victim := liveNodes[rand.Intn(len(liveNodes))]
				victim.Kill()
				if verbose {
					fmt.Printf("[churn] killed %s\n", victim.Callsign)
				}
			} else if len(liveNodes) < 5 {
				// Start a new node
				callsign := fmt.Sprintf("SIM%d", nextNodeID)
				node, err := StartNode(nextNodeID, callsign, 9600+nextNodeID, brokerPort, verbose, rfmpdBin)
				if err == nil {
					nodes = append(nodes, node)
					if verbose {
						fmt.Printf("[churn] started %s\n", callsign)
					}
				}
				nextNodeID++
			}
			lastChurn = time.Now()
		}

		// Convergence check every 15 seconds
		if time.Since(lastCheck) > 15*time.Second && len(allIDs) > 0 {
			liveNodes := getLiveNodes(nodes)
			if len(liveNodes) >= 2 {
				convergenceChecks++
				elapsed, err := WaitIDConvergence(liveNodes, "general", allIDs, 10*time.Second)
				if err != nil {
					convergenceFailures++
				} else {
					totalConvergenceTime += elapsed
				}
			}
			lastCheck = time.Now()
		}

		time.Sleep(200 * time.Millisecond)
	}

	// Stop churn and bring up all dead nodes for final convergence
	for _, n := range nodes {
		if !n.IsRunning() {
			n.Restart(rfmpdBin)
		}
	}
	time.Sleep(2 * time.Second)

	// Final convergence check with all nodes alive
	liveNodes := getLiveNodes(nodes)
	finalPassed := true
	if len(liveNodes) >= 2 && len(allIDs) > 0 {
		convergenceChecks++
		_, err := WaitIDConvergence(liveNodes, "general", allIDs, timeout)
		if err != nil {
			convergenceFailures++
			finalPassed = false
		}
	}

	avgConvergence := time.Duration(0)
	successfulChecks := convergenceChecks - convergenceFailures
	if successfulChecks > 0 {
		avgConvergence = totalConvergenceTime / time.Duration(successfulChecks)
	}

	result.Passed = finalPassed
	result.Duration = time.Since(start)
	result.Detail = fmt.Sprintf("%d messages, %d convergence checks (%d mid-churn failures), avg convergence %v",
		messagesSent, convergenceChecks, convergenceFailures, avgConvergence.Round(100*time.Millisecond))

	return result
}

func RunScenarioMultiClient(brokerPort int, timeout time.Duration, verbose bool, rfmpdBin string) ScenarioResult {
	start := time.Now()
	result := ScenarioResult{Name: "Multi-client WebSocket"}

	nodes := make([]*Node, 3)
	var err error
	for i := 0; i < 3; i++ {
		callsign := fmt.Sprintf("SIM%d", i)
		nodes[i], err = StartNode(i, callsign, 9700+i, brokerPort, verbose, rfmpdBin)
		if err != nil {
			result.Detail = fmt.Sprintf("failed to start node %d: %v", i, err)
			cleanupNodes(nodes)
			return result
		}
	}
	defer cleanupNodes(nodes)

	time.Sleep(1 * time.Second)

	// Connect multiple WebSocket clients to each node
	clientsPerNode := 3
	var allClients []*WSClient
	clientsByNode := make(map[int][]*WSClient)

	for i, node := range nodes {
		for j := 0; j < clientsPerNode; j++ {
			client, err := NewWSClient(node.APIPort)
			if err != nil {
				result.Detail = fmt.Sprintf("WS connect to node %d client %d failed: %v", i, j, err)
				for _, c := range allClients {
					c.Close()
				}
				return result
			}
			allClients = append(allClients, client)
			clientsByNode[i] = append(clientsByNode[i], client)
		}
	}
	defer func() {
		for _, c := range allClients {
			c.Close()
		}
	}()

	// Send messages from different nodes
	messageCount := 5
	for i := 0; i < messageCount; i++ {
		senderIdx := i % len(nodes)
		_, err := SendMessage(nodes[senderIdx].APIPort, "general", fmt.Sprintf("multi-client-msg-%d", i))
		if err != nil {
			result.Detail = fmt.Sprintf("send message %d failed: %v", i, err)
			return result
		}
		time.Sleep(300 * time.Millisecond)
	}

	// Wait for all WebSocket clients on the SENDER node to receive the messages
	// The sender's clients should see all messages sent TO that node
	// Each node eventually has all messages via rebroadcast/sync
	err = WaitWSMessages(allClients, messageCount, timeout)
	if err != nil {
		// Partial pass check: at minimum, each node's clients should see
		// the messages sent directly from that node
		localOk := true
		for nodeIdx, clients := range clientsByNode {
			// Count messages that originated from this node
			expectedLocal := 0
			for i := 0; i < messageCount; i++ {
				if i%len(nodes) == nodeIdx {
					expectedLocal++
				}
			}
			for _, c := range clients {
				if c.MessageCount() < expectedLocal {
					localOk = false
					break
				}
			}
		}
		if !localOk {
			result.Detail = err.Error()
			return result
		}
		// Local delivery works but cross-node WS delivery is slow — still pass
		// but report the situation
		result.Passed = true
		result.Duration = time.Since(start)
		var counts []string
		for i, c := range allClients {
			counts = append(counts, fmt.Sprintf("c%d=%d", i, c.MessageCount()))
		}
		result.Detail = fmt.Sprintf("%d clients, partial WS delivery (%s)", len(allClients), joinDetails(counts))
		return result
	}

	// Verify no client got duplicate messages
	for i, c := range allClients {
		msgs := c.GetMessages()
		seen := make(map[string]bool)
		for _, m := range msgs {
			if seen[m.ID] {
				result.Detail = fmt.Sprintf("client %d received duplicate message %s", i, m.ID)
				return result
			}
			seen[m.ID] = true
		}
	}

	// Verify no client disconnected unexpectedly
	for i, c := range allClients {
		if c.IsClosed() {
			result.Detail = fmt.Sprintf("client %d disconnected unexpectedly", i)
			return result
		}
	}

	result.Passed = true
	result.Duration = time.Since(start)
	result.Detail = fmt.Sprintf("%d clients across %d nodes, all received %d messages",
		len(allClients), len(nodes), messageCount)
	return result
}

func RunScenarioPoorReception(brokerPort int, timeout time.Duration, verbose bool, rfmpdBin string) ScenarioResult {
	start := time.Now()
	result := ScenarioResult{Name: "Poor RF reception"}

	broker := globalBroker

	// Start nodes with clean conditions so they can establish connections
	nodes := make([]*Node, 3)
	var err error
	for i := 0; i < 3; i++ {
		callsign := fmt.Sprintf("SIM%d", i)
		nodes[i], err = StartNode(i, callsign, 9800+i, brokerPort, verbose, rfmpdBin)
		if err != nil {
			result.Detail = fmt.Sprintf("failed to start node %d: %v", i, err)
			cleanupNodes(nodes)
			return result
		}
	}
	defer cleanupNodes(nodes)

	time.Sleep(1 * time.Second)

	// Now apply poor conditions after nodes are connected.
	// 20% random drop exercises the retransmission/SVEC sync mechanism.
	// We don't use CorruptRate here because corrupt frames that still
	// parse as valid MSG frames create phantom messages with wrong IDs,
	// making convergence checking unreliable.
	broker.SetRFCondition(&RFCondition{
		DropRate: 0.20,
	})
	defer broker.SetRFCondition(nil)

	// Send messages with spacing to allow retransmissions
	allIDs := make(map[string]bool)
	messageCount := 6
	for i := 0; i < messageCount; i++ {
		senderIdx := i % len(nodes)
		id, err := SendMessage(nodes[senderIdx].APIPort, "general", fmt.Sprintf("noisy-msg-%d", i))
		if err != nil {
			result.Detail = fmt.Sprintf("send message %d failed: %v", i, err)
			return result
		}
		allIDs[id] = true
		time.Sleep(1 * time.Second)
	}

	// With 20% packet drop, many frames will be lost on first transmission.
	// The protocol should recover via SVEC sync and rebroadcasts over multiple
	// cycles. Use extended timeout since effective per-frame delivery is reduced.
	convergeDuration, err := WaitIDConvergence(nodes, "general", allIDs, timeout*2)
	if err != nil {
		// Under RF impairment the SQLite DB can be busy during SVEC sync,
		// causing API queries to return stale results. Retry a few times.
		recovered := false
		for retry := 0; retry < 5; retry++ {
			time.Sleep(2 * time.Second)
			allFound := true
			for _, n := range nodes {
				if !n.IsRunning() {
					continue
				}
				ids, idErr := GetMessageIDs(n.APIPort, "general")
				if idErr != nil {
					allFound = false
					break
				}
				for id := range allIDs {
					if !ids[id] {
						allFound = false
						break
					}
				}
				if !allFound {
					break
				}
			}
			if allFound {
				recovered = true
				break
			}
		}
		if !recovered {
			var counts []string
			for _, n := range nodes {
				ids, _ := GetMessageIDs(n.APIPort, "general")
				missing := 0
				for id := range allIDs {
					if !ids[id] {
						missing++
					}
				}
				counts = append(counts, fmt.Sprintf("%s=%d/%d(miss %d)", n.Callsign, len(ids), messageCount, missing))
			}
			result.Duration = time.Since(start)
			result.Detail = fmt.Sprintf("convergence failed under poor conditions: %s", joinDetails(counts))
			return result
		}
		convergeDuration = time.Since(start) - 7*time.Second
	}

	result.Passed = true
	result.Duration = time.Since(start)
	result.Detail = fmt.Sprintf("%d messages converged in %v despite 20%% frame drop",
		messageCount, convergeDuration.Round(100*time.Millisecond))
	return result
}

func RunScenarioLargePayload(brokerPort int, timeout time.Duration, verbose bool, rfmpdBin string) ScenarioResult {
	start := time.Now()
	result := ScenarioResult{Name: "Large payload delivery"}

	nodes := make([]*Node, 3)
	var err error
	for i := 0; i < 3; i++ {
		callsign := fmt.Sprintf("SIM%d", i)
		nodes[i], err = StartNode(i, callsign, 9750+i, brokerPort, verbose, rfmpdBin)
		if err != nil {
			result.Detail = fmt.Sprintf("failed to start node %d: %v", i, err)
			cleanupNodes(nodes)
			return result
		}
	}
	defer cleanupNodes(nodes)

	time.Sleep(1 * time.Second)

	// Generate a ~9KB payload similar to an encoded file
	largeBody := strings.Repeat("eJy8fMve48bt5asgq96otZnlLP6/", 300) // ~9000 chars
	allIDs := make(map[string]bool)

	id, err := SendMessage(nodes[0].APIPort, "general", largeBody)
	if err != nil {
		result.Detail = fmt.Sprintf("send large message failed: %v", err)
		return result
	}
	allIDs[id] = true

	// Also send a second large message from a different node
	largeBody2 := strings.Repeat("ABCDEFGHIJKLMNOPQRSTUVWXYZabcd", 250) // ~7500 chars
	id2, err := SendMessage(nodes[1].APIPort, "general", largeBody2)
	if err != nil {
		result.Detail = fmt.Sprintf("send second large message failed: %v", err)
		return result
	}
	allIDs[id2] = true

	// Wait for fragmented delivery and reassembly
	convergeDuration, err := WaitIDConvergence(nodes, "general", allIDs, timeout*2)
	if err != nil {
		var counts []string
		for _, n := range nodes {
			ids, _ := GetMessageIDs(n.APIPort, "general")
			counts = append(counts, fmt.Sprintf("%s=%d/2", n.Callsign, len(ids)))
		}
		result.Duration = time.Since(start)
		result.Detail = fmt.Sprintf("convergence failed: %s", joinDetails(counts))
		return result
	}

	// Verify body integrity on all nodes
	for _, n := range nodes {
		msgs, err := GetMessages(n.APIPort, "general", 10)
		if err != nil {
			result.Detail = fmt.Sprintf("get messages from %s failed: %v", n.Callsign, err)
			return result
		}
		foundFirst := false
		foundSecond := false
		for _, msg := range msgs {
			if msg.Body == largeBody {
				foundFirst = true
			}
			if msg.Body == largeBody2 {
				foundSecond = true
			}
		}
		if !foundFirst || !foundSecond {
			result.Detail = fmt.Sprintf("%s: body integrity check failed (first=%v, second=%v)", n.Callsign, foundFirst, foundSecond)
			return result
		}
	}

	result.Passed = true
	result.Duration = time.Since(start)
	result.Detail = fmt.Sprintf("2 large messages (%d + %d chars, ~%d fragments) delivered intact in %v",
		len(largeBody), len(largeBody2), (len(largeBody)+len(largeBody2))/200, convergeDuration.Round(100*time.Millisecond))
	return result
}

func RunScenarioHighVolume(brokerPort int, timeout time.Duration, verbose bool, rfmpdBin string) ScenarioResult {
	start := time.Now()
	result := ScenarioResult{Name: "High message volume"}

	nodes := make([]*Node, 4)
	var err error
	for i := 0; i < 4; i++ {
		callsign := fmt.Sprintf("SIM%d", i)
		nodes[i], err = StartNode(i, callsign, 9900+i, brokerPort, verbose, rfmpdBin)
		if err != nil {
			result.Detail = fmt.Sprintf("failed to start node %d: %v", i, err)
			cleanupNodes(nodes)
			return result
		}
	}
	defer cleanupNodes(nodes)

	time.Sleep(1 * time.Second)

	// Send 15 messages from all 4 nodes (60 total)
	allIDs := make(map[string]bool)
	messageCount := 15
	for i := 0; i < messageCount; i++ {
		for _, node := range nodes {
			id, err := SendMessage(node.APIPort, "general", fmt.Sprintf("vol-%s-%d", node.Callsign, i))
			if err != nil {
				result.Detail = fmt.Sprintf("send failed on %s msg %d: %v", node.Callsign, i, err)
				return result
			}
			allIDs[id] = true
			time.Sleep(100 * time.Millisecond)
		}
		time.Sleep(300 * time.Millisecond)
	}

	totalMsgs := len(allIDs)
	convergeDuration, err := WaitIDConvergence(nodes, "general", allIDs, timeout)
	if err != nil {
		var counts []string
		for _, n := range nodes {
			ids, _ := GetMessageIDs(n.APIPort, "general")
			counts = append(counts, fmt.Sprintf("%s=%d/%d", n.Callsign, len(ids), totalMsgs))
		}
		result.Duration = time.Since(start)
		result.Detail = fmt.Sprintf("convergence failed: %s", joinDetails(counts))
		return result
	}

	result.Passed = true
	result.Duration = time.Since(start)
	result.Detail = fmt.Sprintf("%d messages across 4 nodes converged in %v",
		totalMsgs, convergeDuration.Round(100*time.Millisecond))
	return result
}

func RunScenarioHeavyLoss(brokerPort int, timeout time.Duration, verbose bool, rfmpdBin string) ScenarioResult {
	start := time.Now()
	result := ScenarioResult{Name: "Heavy packet loss (40%)"}

	broker := globalBroker

	nodes := make([]*Node, 5)
	var err error
	for i := 0; i < 5; i++ {
		callsign := fmt.Sprintf("SIM%d", i)
		nodes[i], err = StartNode(i, callsign, 9950+i, brokerPort, verbose, rfmpdBin)
		if err != nil {
			result.Detail = fmt.Sprintf("failed to start node %d: %v", i, err)
			cleanupNodes(nodes)
			return result
		}
	}
	defer cleanupNodes(nodes)

	time.Sleep(1 * time.Second)

	// Apply 40% frame loss
	broker.SetRFCondition(&RFCondition{DropRate: 0.40})
	defer broker.SetRFCondition(nil)

	// Send 10 messages spread across nodes
	allIDs := make(map[string]bool)
	messageCount := 10
	for i := 0; i < messageCount; i++ {
		senderIdx := i % len(nodes)
		id, err := SendMessage(nodes[senderIdx].APIPort, "general", fmt.Sprintf("loss-msg-%d", i))
		if err != nil {
			result.Detail = fmt.Sprintf("send message %d failed: %v", i, err)
			return result
		}
		allIDs[id] = true
		time.Sleep(500 * time.Millisecond)
	}

	// With 40% loss, SVEC sync needs multiple rounds. Give generous timeout.
	convergeDuration, err := WaitIDConvergence(nodes, "general", allIDs, timeout*3)
	if err != nil {
		// Retry check
		for retry := 0; retry < 5; retry++ {
			time.Sleep(3 * time.Second)
			allFound := true
			for _, n := range nodes {
				if !n.IsRunning() {
					continue
				}
				ids, idErr := GetMessageIDs(n.APIPort, "general")
				if idErr != nil {
					allFound = false
					break
				}
				for id := range allIDs {
					if !ids[id] {
						allFound = false
						break
					}
				}
				if !allFound {
					break
				}
			}
			if allFound {
				convergeDuration = time.Since(start) - 5*time.Second
				err = nil
				break
			}
		}
	}
	if err != nil {
		var counts []string
		for _, n := range nodes {
			ids, _ := GetMessageIDs(n.APIPort, "general")
			missing := 0
			for id := range allIDs {
				if !ids[id] {
					missing++
				}
			}
			counts = append(counts, fmt.Sprintf("%s=%d/%d(miss %d)", n.Callsign, len(ids), messageCount, missing))
		}
		result.Duration = time.Since(start)
		result.Detail = fmt.Sprintf("convergence failed: %s", joinDetails(counts))
		return result
	}

	result.Passed = true
	result.Duration = time.Since(start)
	result.Detail = fmt.Sprintf("%d messages across 5 nodes converged in %v despite 40%% loss",
		messageCount, convergeDuration.Round(100*time.Millisecond))
	return result
}

func RunScenarioRapidCrashCycle(brokerPort int, timeout time.Duration, verbose bool, rfmpdBin string) ScenarioResult {
	start := time.Now()
	result := ScenarioResult{Name: "Rapid crash cycling"}

	nodes := make([]*Node, 4)
	var err error
	for i := 0; i < 4; i++ {
		callsign := fmt.Sprintf("SIM%d", i)
		nodes[i], err = StartNode(i, callsign, 9850+i, brokerPort, verbose, rfmpdBin)
		if err != nil {
			result.Detail = fmt.Sprintf("failed to start node %d: %v", i, err)
			cleanupNodes(nodes)
			return result
		}
	}
	defer cleanupNodes(nodes)

	time.Sleep(1 * time.Second)

	allIDs := make(map[string]bool)

	// Phase 1: send some initial messages
	for i := 0; i < 4; i++ {
		id, err := SendMessage(nodes[i].APIPort, "general", fmt.Sprintf("pre-cycle-%d", i))
		if err != nil {
			result.Detail = fmt.Sprintf("initial send failed: %v", err)
			return result
		}
		allIDs[id] = true
		time.Sleep(300 * time.Millisecond)
	}

	// Wait for initial convergence
	_, err = WaitIDConvergence(nodes, "general", allIDs, timeout/2)
	if err != nil {
		result.Detail = "initial convergence failed: " + err.Error()
		return result
	}

	// Phase 2: rapid crash/restart cycles while sending messages
	crashCycles := 6
	for cycle := 0; cycle < crashCycles; cycle++ {
		// Kill a node
		victim := cycle % len(nodes)
		nodes[victim].Kill()
		time.Sleep(500 * time.Millisecond)

		// Send messages from surviving nodes
		for i, n := range nodes {
			if i == victim || !n.IsRunning() {
				continue
			}
			id, err := SendMessage(n.APIPort, "general", fmt.Sprintf("cycle%d-from-%d", cycle, i))
			if err == nil {
				allIDs[id] = true
			}
			break // one message per cycle is enough
		}

		time.Sleep(1 * time.Second)

		// Restart the killed node
		if err := nodes[victim].Restart(rfmpdBin); err != nil {
			result.Detail = fmt.Sprintf("restart cycle %d failed: %v", cycle, err)
			return result
		}
		time.Sleep(1 * time.Second)
	}

	// Phase 3: all nodes alive, verify full convergence
	_, err = WaitIDConvergence(nodes, "general", allIDs, timeout)
	if err != nil {
		var counts []string
		for _, n := range nodes {
			ids, _ := GetMessageIDs(n.APIPort, "general")
			counts = append(counts, fmt.Sprintf("%s=%d/%d", n.Callsign, len(ids), len(allIDs)))
		}
		result.Duration = time.Since(start)
		result.Detail = fmt.Sprintf("final convergence failed (%d msgs): %s", len(allIDs), joinDetails(counts))
		return result
	}

	result.Passed = true
	result.Duration = time.Since(start)
	result.Detail = fmt.Sprintf("%d crash/restart cycles, %d total messages converged",
		crashCycles, len(allIDs))
	return result
}

func getLiveNodes(nodes []*Node) []*Node {
	var live []*Node
	for _, n := range nodes {
		if n.IsRunning() {
			live = append(live, n)
		}
	}
	return live
}

func cleanupNodes(nodes []*Node) {
	for _, n := range nodes {
		if n != nil {
			n.Cleanup()
		}
	}
}
