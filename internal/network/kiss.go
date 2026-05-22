package network

const (
	FEND  byte = 0xC0
	FESC  byte = 0xDB
	TFEND byte = 0xDC
	TFESC byte = 0xDD
)

type KISSCommand byte

const (
	KISSDataFrame   KISSCommand = 0x00
	KISSTxDelay     KISSCommand = 0x01
	KISSPersistence KISSCommand = 0x02
	KISSSlotTime    KISSCommand = 0x03
	KISSTxTail      KISSCommand = 0x04
	KISSFullDuplex  KISSCommand = 0x05
	KISSSetHardware KISSCommand = 0x06
	KISSReturn      KISSCommand = 0x0F
)

func isValidKISSCommand(b byte) bool {
	switch KISSCommand(b) {
	case KISSDataFrame, KISSTxDelay, KISSPersistence, KISSSlotTime,
		KISSTxTail, KISSFullDuplex, KISSSetHardware, KISSReturn:
		return true
	}
	return false
}

type KISSFrame struct {
	Port    int
	Command KISSCommand
	Data    []byte
}

func (kf *KISSFrame) Encode() []byte {
	cmdByte := byte(kf.Port<<4) | byte(kf.Command)

	content := append([]byte{cmdByte}, kf.Data...)

	escaped := make([]byte, 0, len(content)*2)
	for _, b := range content {
		switch b {
		case FEND:
			escaped = append(escaped, FESC, TFEND)
		case FESC:
			escaped = append(escaped, FESC, TFESC)
		default:
			escaped = append(escaped, b)
		}
	}

	result := make([]byte, 0, len(escaped)+2)
	result = append(result, FEND)
	result = append(result, escaped...)
	result = append(result, FEND)
	return result
}

func DecodeKISSFrame(data []byte) *KISSFrame {
	if len(data) == 0 {
		return nil
	}

	// Strip FEND delimiters
	if data[0] == FEND {
		data = data[1:]
	}
	if len(data) > 0 && data[len(data)-1] == FEND {
		data = data[:len(data)-1]
	}
	if len(data) == 0 {
		return nil
	}

	// Unescape
	unescaped := make([]byte, 0, len(data))
	i := 0
	for i < len(data) {
		if data[i] == FESC {
			if i+1 >= len(data) {
				return nil // Incomplete escape
			}
			switch data[i+1] {
			case TFEND:
				unescaped = append(unescaped, FEND)
				i += 2
			case TFESC:
				unescaped = append(unescaped, FESC)
				i += 2
			default:
				return nil // Invalid escape sequence
			}
		} else {
			unescaped = append(unescaped, data[i])
			i++
		}
	}

	if len(unescaped) == 0 {
		return nil
	}

	cmdByte := unescaped[0]
	port := int((cmdByte >> 4) & 0x0F)
	cmd := cmdByte & 0x0F

	if !isValidKISSCommand(cmd) {
		return nil
	}

	return &KISSFrame{
		Port:    port,
		Command: KISSCommand(cmd),
		Data:    unescaped[1:],
	}
}

type KISSProtocol struct {
	Port   int
	buffer []byte
}

func NewKISSProtocol(port int) *KISSProtocol {
	return &KISSProtocol{Port: port}
}

func (kp *KISSProtocol) EncodeData(data []byte) []byte {
	frame := &KISSFrame{
		Port:    kp.Port,
		Command: KISSDataFrame,
		Data:    data,
	}
	return frame.Encode()
}

func (kp *KISSProtocol) Reset() {
	kp.buffer = nil
}

func (kp *KISSProtocol) DecodeFrames(data []byte) []*KISSFrame {
	kp.buffer = append(kp.buffer, data...)
	var frames []*KISSFrame

	for {
		startIdx := -1
		for i, b := range kp.buffer {
			if b == FEND {
				startIdx = i
				break
			}
		}
		if startIdx < 0 {
			kp.buffer = nil
			break
		}
		if startIdx > 0 {
			kp.buffer = kp.buffer[startIdx:]
			startIdx = 0
		}

		endIdx := -1
		for i := startIdx + 1; i < len(kp.buffer); i++ {
			if kp.buffer[i] == FEND {
				endIdx = i
				break
			}
		}
		if endIdx < 0 {
			break
		}

		frameData := kp.buffer[startIdx : endIdx+1]
		kp.buffer = kp.buffer[endIdx+1:]

		frame := DecodeKISSFrame(frameData)
		if frame != nil && frame.Command == KISSDataFrame {
			frames = append(frames, frame)
		}
	}

	return frames
}
