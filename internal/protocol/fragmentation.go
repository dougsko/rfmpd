package protocol

import (
	"math"
	"sync"
	"time"
)

const fragOverhead = 15

const MaxFragments = 255

type FragmentCollector struct {
	MessageID      [6]byte
	TotalFragments int
	Fragments      map[int][]byte
	FirstSeen      time.Time
	Timeout        time.Duration
}

func NewFragmentCollector(messageID [6]byte, total int) *FragmentCollector {
	return &FragmentCollector{
		MessageID:      messageID,
		TotalFragments: total,
		Fragments:      make(map[int][]byte),
		FirstSeen:      time.Now().UTC(),
		Timeout:        5 * time.Minute,
	}
}

func (fc *FragmentCollector) AddFragment(frag *FRAG) bool {
	if frag.MessageID != fc.MessageID {
		return false
	}
	if frag.Total != fc.TotalFragments {
		return false
	}
	if _, exists := fc.Fragments[frag.Idx]; exists {
		return false
	}
	fc.Fragments[frag.Idx] = frag.Data
	return true
}

func (fc *FragmentCollector) IsComplete() bool {
	return len(fc.Fragments) == fc.TotalFragments
}

func (fc *FragmentCollector) IsExpired() bool {
	return time.Since(fc.FirstSeen) > fc.Timeout
}

func (fc *FragmentCollector) GetMissingIndexes() []int {
	var missing []int
	for i := 0; i < fc.TotalFragments; i++ {
		if _, ok := fc.Fragments[i]; !ok {
			missing = append(missing, i)
		}
	}
	return missing
}

func (fc *FragmentCollector) Reassemble() []byte {
	if !fc.IsComplete() {
		return nil
	}
	var result []byte
	for i := 0; i < fc.TotalFragments; i++ {
		result = append(result, fc.Fragments[i]...)
	}
	return result
}

// Fragmenter splits large messages into FRAG frames and reassembles received fragments.
type Fragmenter struct {
	Threshold  int
	mu         sync.Mutex
	Collectors map[string]*FragmentCollector
}

func NewFragmenter(threshold int) *Fragmenter {
	return &Fragmenter{
		Threshold:  threshold,
		Collectors: make(map[string]*FragmentCollector),
	}
}

func (f *Fragmenter) FragmentMessage(msg *MSG) []*FRAG {
	encoded := EncodeMsgRaw(msg)
	if len(encoded) <= f.Threshold {
		return nil
	}

	fragmentSize := max(1, f.Threshold-fragOverhead)
	totalFragments := int(math.Ceil(float64(len(encoded)) / float64(fragmentSize)))

	frags := make([]*FRAG, 0, totalFragments)
	for i := 0; i < totalFragments; i++ {
		start := i * fragmentSize
		end := start + fragmentSize
		if end > len(encoded) {
			end = len(encoded)
		}
		frags = append(frags, &FRAG{
			MessageID: msg.ID,
			Idx:       i,
			Total:     totalFragments,
			Data:      encoded[start:end],
		})
	}
	return frags
}

func (f *Fragmenter) AddFragment(frag *FRAG) (isNew bool, reassembled *MSG) {
	if frag.Total > MaxFragments {
		return false, nil
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	key := IDToHex(frag.MessageID)
	collector, ok := f.Collectors[key]
	if !ok {
		collector = NewFragmentCollector(frag.MessageID, frag.Total)
		f.Collectors[key] = collector
	}

	isNew = collector.AddFragment(frag)

	if collector.IsComplete() {
		data := collector.Reassemble()
		if data != nil {
			msg, err := DecodeMsgRaw(data)
			if err == nil {
				delete(f.Collectors, key)
				return isNew, msg
			}
		}
	}

	return isNew, nil
}

func (f *Fragmenter) CleanupExpired() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	var expired []string
	for id, collector := range f.Collectors {
		if collector.IsExpired() {
			expired = append(expired, id)
			delete(f.Collectors, id)
		}
	}
	return expired
}

