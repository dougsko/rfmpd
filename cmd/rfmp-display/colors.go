package main

const (
	ColorBlack    uint16 = 0x0000
	ColorWhite    uint16 = 0xFFFF
	ColorBlue     uint16 = 0x1E9F
	ColorDarkGray uint16 = 0x2104
	ColorLightGray uint16 = 0xC618
	ColorGreen    uint16 = 0x07E0
	ColorRed      uint16 = 0xF800
	ColorYellow   uint16 = 0xFFE0
)

func RGB(r, g, b uint8) uint16 {
	return uint16(r>>3)<<11 | uint16(g>>2)<<5 | uint16(b>>3)
}
