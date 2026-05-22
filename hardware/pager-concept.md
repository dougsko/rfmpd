# RFMP Pager — Standalone Hardware Build

A self-contained RF microblogging device: no phone required.
Plug into a radio via Digirig and go.

## Architecture

```
┌─────────────────────────────┐
│   2.4" ILI9341 IPS (SPI)    │
├─────────────────────────────┤
│   BBQ20KBD Keyboard (I2C)   │
│   + optical trackpad         │
├─────────────────────────────┤
│   Orange Pi Zero 2W          │
│   ├─ rfmpd (daemon)         │
│   ├─ rfmp-display (UI)      │
│   ├─ Direwolf (modem)       │
│   └─ WiFi AP (optional)     │
├─────────────────────────────┤
│   LiPo 3000mAh + IP5306     │
└─────────────────────────────┘
        │ USB audio
        ▼
   ┌──────────┐
   │ Digirig  │──── audio cable ───► Radio
   └──────────┘
```

## Bill of Materials

| # | Component | Part | Source | Price |
|---|-----------|------|--------|-------|
| 1 | SBC | Orange Pi Zero 2W (1GB) | AliExpress/Amazon | $22 |
| 2 | Display | 2.4" ILI9341 240×320 IPS, SPI | AliExpress | $8 |
| 3 | Keyboard | Solder Party BBQ20KBD (BB Q20 + trackpad) | Tindie | $30 |
| 4 | Audio interface | Digirig Mobile | digirig.net | $45 |
| 5 | Radio cable | Digirig cable (model-specific) | digirig.net | $15 |
| 6 | Battery | 3.7V 3000mAh LiPo flat pack | Amazon/AliExpress | $10 |
| 7 | Power management | IP5306 USB-C power bank module (charge + boost) | AliExpress | $3 |
| 8 | Enclosure | 3D printed custom (SLA resin or MJF nylon) | JLCPCB/PCBWay | $15 |
| 9 | Storage | MicroSD 32GB | Amazon | $5 |
| 10 | Misc | JST pigtails, headers, screws, standoffs, LED, 220Ω resistor | Amazon | $5 |
| | **Total** | | | **~$158** |

### Prebuilt Alternative: HackberryPi

Instead of sourcing an enclosure, SBC, display, and keyboard separately, the
**HackberryPi Cyberdeck with Q20 Keyboard** ($127 on Tindie) is a fully
assembled handheld that includes:

- Raspberry Pi Zero 2W + 16GB microSD (OS pre-installed)
- 4" 720×720 TFT display
- BB Q20 keyboard with trackpad
- 3D-printed enclosure
- 3× USB 2.0, HDMI, Stemma I2C port
- Dual swappable Nokia BL-5C batteries

You'd just need to add the Digirig ($45) + radio cable ($15) and install
rfmpd + rfmp-display + Direwolf. Total: ~$187 but zero custom fabrication —
it's ready to go out of the box. Trade-off: larger form factor than a custom
build, and the 4" display is different from the 240×320 ILI9341 that
rfmp-display currently targets (would need UI resolution adjustment).

## Orange Pi Zero 2W Specs

- **Dimensions:** 65 × 30 mm (RPi Zero 2W form factor)
- **SoC:** Allwinner H618
- **RAM:** 1GB
- **WiFi:** 802.11 b/g/n 2.4GHz + Bluetooth 4.0 (onboard)
- **Power:** USB-C 5V/2A
- **Header:** 40-pin GPIO
- **OS:** Armbian (Debian-based)

## Wiring

### Display (2.4" ILI9341) → Orange Pi Zero 2W (SPI0)

| Display Pin | Function | OPi Pin # | OPi Signal | H618 Pad |
|-------------|----------|-----------|------------|----------|
| VCC | Power | 1 | 3.3V | — |
| GND | Ground | 6 | GND | — |
| SCL/CLK | SPI clock | 23 | SPI0_SCLK | PH6 |
| SDA/MOSI | SPI data | 19 | SPI0_MOSI | PH7 |
| CS | Chip select | 24 | SPI0_CE0 | PH5 |
| DC/RS | Data/Command | 18 | GPIO24 | PH4 |
| RST | Reset | 12 | GPIO18 | PI1 |
| LED/BLK | Backlight | 17 | 3.3V (always on) | — |

Linux device: `/dev/spidev0.0`

**Module PCB size:** ~71 × 52 mm

### Keyboard (BBQ20KBD) → Orange Pi Zero 2W (I2C1)

| KB Pin | Function | OPi Pin # | OPi Signal | H618 Pad |
|--------|----------|-----------|------------|----------|
| 3V3 | Power | 1 | 3.3V | — |
| GND | Ground | 9 | GND | — |
| SDA | I2C data | 3 | I2C1_SDA | PI8 |
| SCL | I2C clock | 5 | I2C1_SCL | PI7 |
| INT | Interrupt (active low) | 7 | GPIO4 | PI13 |

I2C address: `0x1F`. Linux device: `/dev/i2c-1`

> The BBQ20KBD is 3.3V only — do NOT connect to 5V.

### Status LED

```
GPIO5 (Pin 29, PI0) → 220Ω → LED anode (+) → LED cathode (−) → GND (Pin 30)
```

### Power Circuit

```
                  ┌──────────────────┐
USB-C ──→ VBUS ──┤                  ├──→ 5V out ──→ OPi 5V (Pin 2)
                  │     IP5306       │             └→ GND (Pin 6/14)
LiPo 3.7V ──────┤  (charge+boost)  │
                  └──────────────────┘
```

The IP5306 is a single-chip power bank IC that replaces the TP4056 + MT3608
combo. It provides:

- **Path management** — powers the load from USB when plugged in, charges the
  battery simultaneously, and correctly terminates charge regardless of load
  current (the TP4056 cannot do this)
- **Integrated boost** — converts 3.7V battery → 5V output, no separate MT3608
- **Battery protection** — over-discharge cutoff, overcurrent, over-temperature
- **LED charge indicators** — 4 outputs for charge level LEDs (optional)
- **USB-C** input — modules with USB-C are readily available on AliExpress

Feed 5V directly to Pin 2 (bypasses Pi's USB-C port).
All 3.3V peripherals (display, keyboard) are powered by the Pi's onboard
regulator.

### Digirig

- Connects to OPi USB port (USB audio device + serial PTT)
- Audio cable from Digirig → radio mic/speaker jack
- Cable is radio-specific (Baofeng K-plug, Kenwood 2-pin, Yaesu, etc.)

### Pin Budget

| Pin # | Used By |
|-------|---------|
| 1 | 3.3V (display + keyboard) |
| 2 | 5V in (battery/boost) |
| 3 | I2C1 SDA (keyboard) |
| 5 | I2C1 SCL (keyboard) |
| 6 | GND (display, power) |
| 7 | Keyboard INT |
| 9 | GND (keyboard) |
| 12 | Display RST |
| 17 | Display backlight (3.3V) |
| 18 | Display DC |
| 19 | SPI0 MOSI (display) |
| 23 | SPI0 SCLK (display) |
| 24 | SPI0 CE0 (display) |
| 29 | Status LED |
| 30 | GND (LED) |

**15 of 40 pins used.** Free for future: I2C2 (27/28), UART (8/10), SPI MISO (21),
plus many GPIOs for GPS, battery gauge, etc.

### Carrier HAT

A small PCB (65×30mm) that plugs onto the 40-pin header and routes all
scattered signals to two JST-XH locking connectors (8-pin display, 5-pin
keyboard). Eliminates counting pins and prevents wires from vibrating loose
in a go-bag. The LED and current-limiting resistor are soldered directly on
the HAT.

See [carrier-hat/](carrier-hat/) for the full KiCad project, BOM, assembly
instructions, and cable shopping list.

### Easiest Connection Path

| Peripheral | Method | What to Buy |
|------------|--------|-------------|
| ILI9341 display | Desolder male header, solder 8-pin JST-XH pigtail into holes | JST-XH 8-pin pigtail 10cm |
| BBQ20KBD keyboard | Qwiic cable for I2C+power + 1 wire for INT (or solder 5-pin pigtail directly) | Qwiic cable 10cm + 1 Dupont wire |
| Status LED | Mounted on carrier HAT (no cable) | Included on board |

Ribbon cables (~8-10cm) run from the lid-mounted peripherals down to the HAT
on the OPi in the bottom of the enclosure. JST-XH connectors lock in place —
no pins to count, no Dupont wires to wiggle loose.

## BBQ20KBD Details

- **Keys:** Full QWERTY BB Q20 layout with Shift/Sym layers
- **Trackpad:** Optical, reports X/Y deltas over I2C (registers 0x15, 0x16)
- **MCU:** RP2040 running i2c_puppet firmware
- **Protocol:** I2C register-based, key events in a FIFO (register 0x09)
- **Connections:** 3.3V, GND, SDA, SCL, INT (active low on key press)
- **Also has:** USB-C (works as USB HID keyboard simultaneously)
- **Dimensions:** ~75 × 55 mm

### Key I2C Registers

| Register | Addr | Purpose |
|----------|------|---------|
| KEY_FIFO | 0x09 | Read 2 bytes: [state, ASCII char] |
| BACKLIGHT | 0x05 | Set keyboard backlight (0x00–0xFF) |
| INT_STATUS | 0x03 | Read what triggered the interrupt |
| TRACKPAD_X | 0x15 | Signed X delta |
| TRACKPAD_Y | 0x16 | Signed Y delta |

## Physical Layout

```
┌─────────────────────────────────────┐
│  ┌───────────────────────────────┐  │
│  │         2.4" Display          │  │  ← top of lid, cutout window
│  └───────────────────────────────┘  │
│  ┌───────────────────────────────┐  │
│  │      BBQ20KBD Keyboard        │  │  ← mounted below display
│  │      [Q][W][E][R][T][Y]...    │  │
│  │         [trackpad]            │  │
│  └───────────────────────────────┘  │
│                                     │
└─────────────────────────────────────┘
  Side cutouts:
  - USB-C (IP5306 charge input)
  - USB-C (OPi USB port → Digirig)
  - 3.5mm audio jack pass-through (optional, for radio cable)

Enclosure: ~104 × 79 × 36 mm 3D printed (see enclosure/)
```

### Enclosure

A parametric OpenSCAD design provides a two-part (box + lid) 3D-printable
enclosure with all cutouts and mounting features built in. See
[enclosure/](enclosure/) for the full design and printing instructions.

Key features:
- Display and keyboard windows pre-cut in the lid
- M2.5 standoff bosses matching OPi/HAT hole pattern (58×23mm)
- Slide-out battery compartment (open one end, retaining lip on the other)
- USB-C and USB port cutouts in side walls
- M3 countersunk corner screws hold lid to box
- Alignment lip for snug fit

The enclosure fits the OPi + carrier HAT stack, IP5306 module, and a
70×40×8mm LiPo cell. Outer dimensions are computed parametrically from
component sizes (~104 × 79 × 36mm with default values).

Order from JLCPCB or PCBWay 3D printing service alongside the carrier HAT
PCB — SLA resin (black, ~$15) or MJF nylon (stronger, ~$20).

## Power Budget

| Component | Typical Draw |
|-----------|-------------|
| OPi Zero 2W (WiFi idle) | 150–200 mA |
| OPi Zero 2W (active) | 250–350 mA |
| ILI9341 + backlight | 30–50 mA |
| Digirig | ~10 mA |
| BBQ20KBD | <5 mA |
| **Total typical** | **~250–350 mA** |

**Battery life:** 3000 mAh ÷ 300 mA avg = **~10 hours**

## Software

### Stack (all auto-start via systemd)

1. **Direwolf** — AX.25 TNC, uses Digirig USB audio
2. **rfmpd** — RFMP daemon (protocol, storage, API on localhost:8080)
3. **rfmp-display** — Hardware UI (new binary in this repo)

### rfmp-display Architecture

Uses a **backend abstraction** so the same UI runs on both macOS (simulator) and Pi (hardware):

```
cmd/rfmp-display/
├── main.go              // flags, signal handling, run loop
├── api.go              // HTTP client: GetChannels, GetMessages, SendMessage, GetStatus
├── ws.go               // WebSocket client (real-time messages from rfmpd)
├── backend.go          // Display, Keyboard, LED interfaces
├── backend_sim.go      // //go:build sim — web canvas on :9090 + browser keyboard
├── backend_hw.go       // //go:build hw — hardware stubs (not yet implemented)
├── framebuffer.go      // RGB565 pixel buffer + drawing primitives
├── font.go             // Embedded 8×16 bitmap font (30 cols × 20 rows)
├── colors.go           // RGB565 color constants
├── ui.go               // Screen state machine
├── ui_channels.go      // Channel list screen
├── ui_timeline.go      // Message timeline screen
├── ui_compose.go       // Compose screen + text input
├── ui_create_channel.go // Channel name input screen
└── sim.html            // Browser canvas renderer
```

**Build:**
- `go build -tags sim` → web canvas on localhost:9090 (develop without hardware)
- `go build -tags hw` → hardware backend stubs (to be implemented when Pi + parts arrive)

**Interfaces:**
```go
type Display interface {
    Init() error
    SetPixel(x, y int, color uint16)
    FillRect(x, y, w, h int, color uint16)
    Flush() error
    Close()
}

type Keyboard interface {
    Init() error
    Events() <-chan KeyEvent
    Close()
}

type LED interface {
    Init() error
    On()
    Off()
    Close()
}
```

**Dependencies:**
- `github.com/gorilla/websocket` (sim + rfmpd client)
- `periph.io/x/conn/v3`, `periph.io/x/host/v3` (hw only)

**Simulator:** Serves a web page on localhost:9090 with a 480×640 canvas (2× scaled 240×320). Browser keyboard maps to BBQ20 key events via WebSocket. LED shows as a colored dot in the page.

**Development order:**
1. Framebuffer + font (text rendering)
2. Backend interfaces + web canvas simulator
3. API + WebSocket client (connect to rfmpd)
4. UI screens (channels → timeline → compose)
5. LED indicator
6. Hardware backend (when Pi + parts arrive)

- Pure API client of rfmpd (same as the web UI)
- REST: `http://localhost:8080` for channels, messages, status
- WebSocket: `ws://localhost:8080/stream` for real-time messages

### UI Flow

```
[Boot splash] → [Channel list] → [Message timeline] → [Compose]
                      ↑                                     │
                      └──────────── Back key ────────────────┘
```

- **Trackpad scroll:** Navigate messages up/down
- **Trackpad click:** Select channel / confirm
- **Letter keys:** Jump to compose, type message
- **Enter:** Send message
- **Back/Esc:** Return to previous screen
- **Status bar:** Channel name, battery %, WiFi indicator, connection status

## Linux Setup

1. Flash Armbian to microSD
2. Edit `/boot/orangepiEnv.txt`:
   ```
   overlays=spi0-cs0 i2c1
   ```
3. Reboot, verify:
   ```bash
   ls /dev/spidev0.0   # display
   ls /dev/i2c-1       # keyboard
   i2cdetect -y 1      # should show 0x1f (BBQ20KBD)
   ```
4. Install Direwolf, configure for Digirig USB audio
5. Enable systemd services: `rfmpd`, `rfmp-display`, `direwolf`

## WiFi AP (optional)

The display and web UI are both just clients of the single rfmpd instance.
Whether the WiFi AP is active is independent of everything else:

- **AP off (default):** Lower power draw, display-only interaction.
- **AP on:** OPi broadcasts a WiFi network; phone connects and uses the web UI at `http://192.168.x.1:8080`.

Controlled by config (`wifi_ap: true/false`) or a key combo on the device (e.g. Sym+W to toggle hostapd).

## Status LED

Single LED on GPIO5 / Pin 29 (PI0) with a 220Ω resistor to GND (Pin 30).
See wiring table above.

| State | LED Behavior |
|-------|-------------|
| Idle | Off |
| Message received | Blink 3× |
| Transmitting | Solid on |

Driven by `rfmp-display` — it already knows both events (WebSocket `message` event for RX, POST response for TX).

## Future Additions

- **GPS** (u-blox NEO-6M, UART on pins 8/10, ~$5) — location-stamped messages
- **Battery gauge** (INA219 on I2C-2, pins 27/28) — accurate % on status bar
