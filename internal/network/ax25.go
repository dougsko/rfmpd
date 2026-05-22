package network

import (
	"fmt"
	"strconv"
	"strings"
)

type AX25Address struct {
	Callsign string
	SSID     int
}

func NewAX25Address(callsign string, ssid int) (*AX25Address, error) {
	callsign = strings.ToUpper(callsign)
	if len(callsign) > 6 {
		return nil, fmt.Errorf("callsign too long: %s", callsign)
	}
	if ssid < 0 || ssid > 15 {
		return nil, fmt.Errorf("SSID must be 0-15, got %d", ssid)
	}
	return &AX25Address{Callsign: callsign, SSID: ssid}, nil
}

func ParseAX25Address(s string) (*AX25Address, error) {
	if idx := strings.Index(s, "-"); idx >= 0 {
		callsign := s[:idx]
		ssidStr := s[idx+1:]
		ssid, err := strconv.Atoi(ssidStr)
		if err != nil {
			return nil, fmt.Errorf("invalid SSID in address: %q", s)
		}
		if ssid < 0 || ssid > 15 {
			return nil, fmt.Errorf("SSID out of range (0-15): %d", ssid)
		}
		return NewAX25Address(callsign, ssid)
	}
	return NewAX25Address(s, 0)
}

func (a *AX25Address) String() string {
	if a.SSID == 0 {
		return a.Callsign
	}
	return fmt.Sprintf("%s-%d", a.Callsign, a.SSID)
}

func (a *AX25Address) Encode(isLast bool) []byte {
	padded := a.Callsign
	for len(padded) < 6 {
		padded += " "
	}

	result := make([]byte, 7)
	for i := 0; i < 6; i++ {
		result[i] = padded[i] << 1
	}

	ssidByte := byte(0x60) | byte(a.SSID<<1)
	if isLast {
		ssidByte |= 0x01
	}
	result[6] = ssidByte

	return result
}

func DecodeAX25Address(data []byte) *AX25Address {
	if len(data) != 7 {
		return nil
	}

	var chars []byte
	for i := 0; i < 6; i++ {
		c := data[i] >> 1
		if c != ' ' {
			chars = append(chars, c)
		}
	}

	ssid := int((data[6] >> 1) & 0x0F)

	return &AX25Address{
		Callsign: string(chars),
		SSID:     ssid,
	}
}

type AX25Frame struct {
	Destination  *AX25Address
	Source       *AX25Address
	Digipeaters  []*AX25Address
	Control      byte
	PID          byte
	Info         []byte
}

func (f *AX25Frame) Encode() []byte {
	var result []byte

	result = append(result, f.Destination.Encode(false)...)

	isLast := len(f.Digipeaters) == 0
	result = append(result, f.Source.Encode(isLast)...)

	for i, digi := range f.Digipeaters {
		isLast = (i == len(f.Digipeaters)-1)
		result = append(result, digi.Encode(isLast)...)
	}

	result = append(result, f.Control)
	result = append(result, f.PID)
	result = append(result, f.Info...)

	return result
}

func DecodeAX25Frame(data []byte) *AX25Frame {
	if len(data) < 16 {
		return nil
	}

	dest := DecodeAX25Address(data[0:7])
	if dest == nil {
		return nil
	}

	src := DecodeAX25Address(data[7:14])
	if src == nil {
		return nil
	}

	var digipeaters []*AX25Address
	idx := 14

	// Check if source has extension bit set
	if data[13]&0x01 == 0 {
		for idx+7 <= len(data) {
			digi := DecodeAX25Address(data[idx : idx+7])
			if digi == nil {
				break
			}
			digipeaters = append(digipeaters, digi)
			idx += 7
			if data[idx-1]&0x01 != 0 {
				break
			}
		}
	}

	if idx >= len(data) {
		return nil
	}
	control := data[idx]
	idx++

	if idx >= len(data) {
		return nil
	}
	pid := data[idx]
	idx++

	info := data[idx:]

	return &AX25Frame{
		Destination: dest,
		Source:      src,
		Digipeaters: digipeaters,
		Control:     control,
		PID:         pid,
		Info:        info,
	}
}

func CreateUIFrame(source, destination string, info []byte) (*AX25Frame, error) {
	src, err := ParseAX25Address(source)
	if err != nil {
		return nil, err
	}
	dst, err := ParseAX25Address(destination)
	if err != nil {
		return nil, err
	}
	return &AX25Frame{
		Destination: dst,
		Source:      src,
		Control:     0x03,
		PID:         0xF0,
		Info:        info,
	}, nil
}
