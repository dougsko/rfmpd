# RFMP Pager Carrier HAT

A small PCB that plugs onto the Orange Pi Zero 2W 40-pin header and breaks out
all peripheral connections to two JST-XH connectors. Eliminates loose Dupont
wires and makes assembly plug-and-play.

## Board Specs

- **Dimensions:** 65 × 30 mm (matches OPi Zero 2W outline)
- **Layers:** 2 (F.Cu + B.Cu)
- **Connector type:** JST-XH (2.5mm pitch, locking, through-hole)
- **Mounting:** 4× M2.5 holes matching OPi standoff positions

## Schematic

```
                        RFMP Pager Carrier HAT
    ┌─────────────────────────────────────────────────────────┐
    │                                                         │
    │   ┌─────────────────────────┐    ┌──────────────────┐  │
    │   │  J1: 2×20 Female Header │    │ J2: DISPLAY      │  │
    │   │  (plugs onto OPi)       │    │ 8-pin JST-XH     │  │
    │   │                         │    │                   │  │
    │   │  Pin 1  ─── 3V3 bus ────┼────┤─ 1: 3V3          │  │
    │   │  Pin 6  ─── GND bus ────┼────┤─ 2: GND          │  │
    │   │  Pin 23 ─── SPI0_SCLK ─┼────┤─ 3: SCLK         │  │
    │   │  Pin 19 ─── SPI0_MOSI ─┼────┤─ 4: MOSI         │  │
    │   │  Pin 24 ─── SPI0_CE0 ──┼────┤─ 5: CS           │  │
    │   │  Pin 18 ─── GPIO24 ────┼────┤─ 6: DC           │  │
    │   │  Pin 12 ─── GPIO18 ────┼────┤─ 7: RST          │  │
    │   │  Pin 17 ─── 3V3 ───────┼────┤─ 8: BLK          │  │
    │   │                         │    └──────────────────┘  │
    │   │                         │    ┌──────────────────┐  │
    │   │                         │    │ J3: KEYBOARD     │  │
    │   │                         │    │ 5-pin JST-XH     │  │
    │   │                         │    │                   │  │
    │   │  Pin 1  ─── 3V3 bus ────┼────┤─ 1: 3V3          │  │
    │   │  Pin 9  ─── GND bus ────┼────┤─ 2: GND          │  │
    │   │  Pin 3  ─── I2C1_SDA ──┼────┤─ 3: SDA          │  │
    │   │  Pin 5  ─── I2C1_SCL ──┼────┤─ 4: SCL          │  │
    │   │  Pin 7  ─── GPIO4 ─────┼────┤─ 5: INT          │  │
    │   │                         │    └──────────────────┘  │
    │   └─────────────────────────┘                          │
    │                                                         │
    │   [D1]──[R1 220Ω]──── Pin 29 (GPIO5)    Status LED    │
    │     │                                                   │
    │     └── GND (Pin 30)                                    │
    │                                                         │
    │   [C1 100nF] across 3V3/GND    Decoupling              │
    │                                                         │
    └─────────────────────────────────────────────────────────┘
```

## Pin Mapping (OPi Header → JST Connectors)

| OPi Pin | Signal | H618 Pad | Routed To |
|---------|--------|----------|-----------|
| 1 | 3.3V | — | J2.1, J3.1 |
| 3 | I2C1_SDA | PI8 | J3.3 |
| 5 | I2C1_SCL | PI7 | J3.4 |
| 6 | GND | — | J2.2 |
| 7 | GPIO4 | PI13 | J3.5 (KB INT) |
| 9 | GND | — | J3.2 |
| 12 | GPIO18 | PI1 | J2.7 (DSP RST) |
| 17 | 3.3V | — | J2.8 (BLK, via 3V3 bus) |
| 18 | GPIO24 | PH4 | J2.6 (DSP DC) |
| 19 | SPI0_MOSI | PH7 | J2.4 |
| 23 | SPI0_SCLK | PH6 | J2.3 |
| 24 | SPI0_CE0 | PH5 | J2.5 |
| 29 | GPIO5 | PI0 | R1 → D1 (LED) |
| 30 | GND | — | D1 cathode |

## Bill of Materials (HAT only)

| Ref | Part | Package | Qty | Notes |
|-----|------|---------|-----|-------|
| J1 | 2×20 female header, 2.54mm | Through-hole | 1 | Plugs onto OPi |
| J2 | JST-XH 8-pin right-angle | B8B-XH-A | 1 | Display |
| J3 | JST-XH 5-pin right-angle | B5B-XH-A | 1 | Keyboard |
| R1 | 220Ω resistor | 0805 or axial | 1 | LED current limit |
| D1 | LED 3mm green | Through-hole | 1 | Status indicator |
| C1 | 100nF ceramic | 0805 | 1 | 3V3 decoupling |

## Fabrication

Order from JLCPCB or PCBWay:
- Upload the Gerber files (export from KiCad: File → Fabrication Outputs → Gerbers)
- Settings: 2-layer, 1.6mm thickness, HASL finish, any color
- Minimum order: 5 boards, ~$5 total + shipping
- Turnaround: 3-5 days fabrication + 5-10 days shipping

## Assembly Notes

### Soldering the HAT

1. Solder J1 (2×20 female header) on the **bottom** of the board — this plugs
   down onto the OPi's male header pins
2. Solder J2, J3 (JST-XH connectors) on the **top**, angled toward the
   board edge for easy cable access
3. Solder R1, D1, C1 on the top side

### Connecting Peripherals

The easiest connection method for each peripheral:

| Peripheral | Connection Method | What to Buy |
|------------|-------------------|-------------|
| ILI9341 display | Solder JST-XH 8-pin pigtail into display header holes | "JST-XH 8-pin pigtail 10cm" |
| BBQ20KBD keyboard | Qwiic cable (4-pin) for I2C+power, plus 1 wire for INT | "Qwiic cable 10cm" + 1× Dupont F-F |
| Status LED | Mounted directly on HAT (D1) | Included on board |

**Detailed steps:**

#### Display (ILI9341)

The display module has 8 through-hole pin positions. Most ship with a
pre-soldered male header — desolder it (or buy the module without header):

1. Get a JST-XH 8-pin pigtail (plug on one end, bare tinned wires on the other)
2. Thread the 8 bare wires through the display's header holes from the back
3. Solder on the front side, trim excess
4. The JST plug end connects to J2 on the HAT

Pin order on the pigtail must match J2: `3V3, GND, SCLK, MOSI, CS, DC, RST, BLK`

Most ILI9341 modules label their pins left-to-right as:
`VCC, GND, CS, RESET, DC, SDI(MOSI), SCK, LED`

So the pigtail wire mapping is:

| Pigtail Wire (J2 pin) | Display Module Pin |
|------------------------|--------------------|
| 1 (3V3) | VCC |
| 2 (GND) | GND |
| 3 (SCLK) | SCK |
| 4 (MOSI) | SDI/MOSI |
| 5 (CS) | CS |
| 6 (DC) | DC |
| 7 (RST) | RESET |
| 8 (BLK) | LED |

Note: module pin order varies by manufacturer — verify your specific module's
silkscreen before soldering. If the order doesn't match, just arrange the
pigtail wires accordingly.

#### Keyboard (BBQ20KBD)

The BBQ20KBD already has a Qwiic/STEMMA QT connector (JST-SH 4-pin, 1mm pitch)
providing I2C + power. Only the INT pin needs a manual wire:

1. Buy a pre-made Qwiic cable (4-pin JST-SH, 10cm)
2. Cut one end off the Qwiic cable, strip the 4 wires
3. Solder those 4 wires to a JST-XH 5-pin pigtail on positions 1-4:
   - Red → pin 1 (3V3)
   - Black → pin 2 (GND)
   - Blue → pin 3 (SDA)
   - Yellow → pin 4 (SCL)
4. For pin 5 (INT): solder a single wire from the BBQ20KBD INT pad to
   position 5 on the same JST-XH pigtail
5. Plug the JST-XH end into J3 on the HAT

Alternative: skip the Qwiic cable entirely and solder a 5-pin JST-XH pigtail
directly to the BBQ20KBD's through-hole pads (3V3, GND, SDA, SCL, INT are all
available as solder points on the board).

### Physical Assembly in Enclosure

```
┌─────────── Enclosure Lid ───────────────┐
│                                         │
│  ┌───────────────────────────────────┐  │
│  │         ILI9341 Display           │  │  ← Mounted with M2 screws
│  └────────────────┬──────────────────┘  │    through module corner holes
│                   │ JST cable (~8cm)    │
│  ┌────────────────┼─────────────────┐   │
│  │   BBQ20KBD Keyboard              │   │  ← KB screwed down
│  │   [Q][W][E][R][T][Y]...          │   │
│  │         [trackpad]               │   │
│  └────────────────┼─────────────────┘   │
│                   │                     │
└───────────────────┼─────────────────────┘
                    │
    JST cables run  │  (~8cm each)
    through gap     │
                    │
┌───────────────────┼─────────────────────┐
│                   │                     │
│  ┌────────────────┴──────────────────┐  │
│  │       Carrier HAT                 │  │
│  │  [J2]  [J3]  ←── cables          │  │  ← JST plugs click in here
│  │       [D1 LED]                    │  │
│  │  ═══════════════════════════════  │  │
│  │  ║  2×20 female header (bottom) ║  │  │
│  └──╨════════════════════════════╨──┘  │
│     ║   Orange Pi Zero 2W        ║     │
│     ║   [USB-C]  [40-pin header] ║     │  ← OPi screwed to bottom
│     ╚════════════════════════════╝     │    standoffs; HAT plugs on top
│                                         │
│  [IP5306 power module]  [LiPo]          │  ← Tucked beside OPi
│                                         │
└─────────── Enclosure Bottom ────────────┘
```

Total solder joints: ~15 (8 display + 5 keyboard + HAT components)
Total cables: 2 JST-XH (click in, click out)

## Shopping List (cables & connectors)

| Item | Qty | Search Term (AliExpress/Amazon) | ~Price |
|------|-----|--------------------------------|--------|
| JST-XH 8-pin pigtail, 10cm | 1 | "JST XH 2.54 8pin wire 10cm" | $0.50 |
| JST-XH 5-pin pigtail, 10cm | 1 | "JST XH 2.54 5pin wire 10cm" | $0.50 |
| Qwiic cable 10cm (optional, for KB) | 1 | "Qwiic STEMMA QT cable 100mm" | $1.50 |

Or buy a JST-XH pigtail assortment pack (~$5 for 20 pieces in various pin counts).

## Project Files

```
hardware/carrier-hat/
├── README.md                # This file — design spec and assembly guide
├── carrier-hat.kicad_pro    # KiCad project
├── carrier-hat.kicad_sch    # KiCad schematic
└── carrier-hat.kicad_pcb    # KiCad PCB outline + placement
```

Open `carrier-hat.kicad_pro` in KiCad 8+. The schematic defines all components
and connectivity. The PCB file has the board outline and mounting holes —
footprints need final placement and trace routing in the interactive router.
Export Gerbers and upload to JLCPCB/PCBWay.
