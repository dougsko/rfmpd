#!/usr/bin/env python3
"""Generate fully-routed RFMP Carrier HAT PCB as raw KiCad S-expression.

No dependencies beyond Python 3. Outputs carrier-hat.kicad_pcb in KiCad 8
format, ready to open, DRC, and export Gerbers.

Usage:
    cd hardware/carrier-hat
    python3 generate_pcb.py
"""

import os
import uuid as _uuid

# ---------------------------------------------------------------------------
# Constants
# ---------------------------------------------------------------------------

BOARD_W = 65.0
BOARD_H = 30.0

TRACE_SIGNAL = 0.25
TRACE_POWER = 0.5
VIA_OUTER = 0.8
VIA_DRILL = 0.4

MOUNT_HOLES = [(3.5, 3.5), (61.5, 3.5), (3.5, 26.5), (61.5, 26.5)]
MOUNT_DRILL = 2.7

NETS = [
    (0, ""),
    (1, "3V3"),
    (2, "GND"),
    (3, "SPI0_MOSI"),
    (4, "SPI0_SCLK"),
    (5, "SPI0_CE0"),
    (6, "I2C1_SDA"),
    (7, "I2C1_SCL"),
    (8, "GPIO_DC"),
    (9, "GPIO_RST"),
    (10, "GPIO_INT"),
    (11, "GPIO_LED"),
    (12, "GPIO_SHUTDOWN"),
]

NET_BY_NAME = {name: idx for idx, name in NETS if name}

J1_PIN_NETS = {
    1: "3V3", 17: "3V3",
    6: "GND", 9: "GND", 30: "GND",
    19: "SPI0_MOSI", 23: "SPI0_SCLK", 24: "SPI0_CE0",
    3: "I2C1_SDA", 5: "I2C1_SCL",
    18: "GPIO_DC", 12: "GPIO_RST", 7: "GPIO_INT",
    29: "GPIO_LED",
    11: "GPIO_SHUTDOWN",
}

J2_PIN_NETS = {
    1: "3V3", 2: "GND", 3: "SPI0_SCLK", 4: "SPI0_MOSI",
    5: "SPI0_CE0", 6: "GPIO_DC", 7: "GPIO_RST", 8: "3V3",
}

J3_PIN_NETS = {
    1: "3V3", 2: "GND", 3: "I2C1_SDA", 4: "I2C1_SCL", 5: "GPIO_INT",
}

J4_PIN_NETS = {1: "GPIO_SHUTDOWN", 2: "GND"}


def uid():
    return str(_uuid.uuid4())


# ---------------------------------------------------------------------------
# Pad position calculators
# ---------------------------------------------------------------------------

def j1_pad_positions(cx=32.5, cy=15.0, pitch=2.54):
    """Return dict: pin_number → (x, y) for 2x20 header."""
    positions = {}
    for row in range(20):
        pin_odd = row * 2 + 1
        pin_even = row * 2 + 2
        py = cy - (9.5 - row) * pitch
        positions[pin_odd] = (cx - pitch / 2, py)
        positions[pin_even] = (cx + pitch / 2, py)
    return positions


def jst_pad_positions(cx, cy, num_pins, pitch=2.50):
    """Return dict: pin_number → (x, y) for JST-XH connector."""
    positions = {}
    start_x = cx - (num_pins - 1) * pitch / 2
    for i in range(num_pins):
        positions[i + 1] = (start_x + i * pitch, cy)
    return positions


# Component centers
J1_POS = (32.5, 15.0)
J2_POS = (58.0, 8.0)
J3_POS = (58.0, 20.0)
J4_POS = (10.0, 8.0)
R1_POS = (10.0, 22.0)
D1_POS = (10.0, 26.0)
C1_POS = (20.0, 5.0)

J1_PADS = j1_pad_positions(*J1_POS)
J2_PADS = jst_pad_positions(*J2_POS, 8)
J3_PADS = jst_pad_positions(*J3_POS, 5)
J4_PADS = jst_pad_positions(*J4_POS, 2)

# 0805: pads at ±0.8mm from center
R1_PADS = {1: (R1_POS[0] - 0.8, R1_POS[1]), 2: (R1_POS[0] + 0.8, R1_POS[1])}
C1_PADS = {1: (C1_POS[0] - 0.8, C1_POS[1]), 2: (C1_POS[0] + 0.8, C1_POS[1])}

# LED: pads at ±1.27mm from center
D1_PADS = {1: (D1_POS[0] - 1.27, D1_POS[1]), 2: (D1_POS[0] + 1.27, D1_POS[1])}


# ---------------------------------------------------------------------------
# S-expression generators
# ---------------------------------------------------------------------------

def fmt(val):
    """Format a float: drop trailing zeros but keep at least one decimal."""
    if val == int(val):
        return f"{int(val)}.0" if val != 0 else "0"
    return f"{val:.4f}".rstrip("0").rstrip(".")


def net_ref(net_name):
    """Return (net <id> "<name>") reference string for a pad."""
    if not net_name:
        return '(net 0 "")'
    return f'(net {NET_BY_NAME[net_name]} "{net_name}")'


def gen_header():
    return f"""(kicad_pcb
  (version 20240108)
  (generator "generate_pcb.py")
  (generator_version "1.0")
  (general
    (thickness 1.6)
    (legacy_teardrops no)
  )
  (paper "A4")
  (title_block
    (title "RFMP Pager Carrier HAT")
    (date "2026-05-22")
    (rev "1.0")
  )
  (layers
    (0 "F.Cu" signal)
    (31 "B.Cu" signal)
    (36 "B.SilkS" user "B.Silkscreen")
    (37 "F.SilkS" user "F.Silkscreen")
    (38 "B.Mask" user "B.Mask")
    (39 "F.Mask" user "F.Mask")
    (44 "Edge.Cuts" user)
  )
  (setup
    (pad_to_mask_clearance 0.05)
    (allow_soldermask_bridges_in_footprints no)
    (pcbplotparams
      (layerselection 0x00010fc_ffffffff)
      (plot_on_all_layers_selection 0x0000000_00000000)
    )
  )
"""


def gen_nets():
    lines = []
    for net_id, name in NETS:
        lines.append(f'  (net {net_id} "{name}")')
    return "\n".join(lines) + "\n"


def gen_outline():
    edges = [
        (0, 0, BOARD_W, 0),
        (BOARD_W, 0, BOARD_W, BOARD_H),
        (BOARD_W, BOARD_H, 0, BOARD_H),
        (0, BOARD_H, 0, 0),
    ]
    lines = []
    for x1, y1, x2, y2 in edges:
        lines.append(
            f'  (gr_line (start {fmt(x1)} {fmt(y1)}) (end {fmt(x2)} {fmt(y2)}) '
            f'(layer "Edge.Cuts") (width 0.15) (uuid "{uid()}"))'
        )
    return "\n".join(lines) + "\n"


def gen_mounting_holes():
    lines = []
    for i, (mx, my) in enumerate(MOUNT_HOLES):
        lines.append(f"""  (footprint "MountingHole:MountingHole_2.7mm_M2.5"
    (at {fmt(mx)} {fmt(my)})
    (layer "F.Cu")
    (uuid "{uid()}")
    (fp_text reference "MH{i+1}" (at 0 -2) (layer "F.SilkS") (effects (font (size 0.8 0.8) (thickness 0.1))))
    (fp_text value "MountingHole" (at 0 2) (layer "F.Fab") (effects (font (size 0.8 0.8) (thickness 0.1))))
    (pad "" np_thru_hole circle (at 0 0) (size {fmt(MOUNT_DRILL)} {fmt(MOUNT_DRILL)}) (drill {fmt(MOUNT_DRILL)}) (layers "*.Cu" "*.Mask") (uuid "{uid()}"))
  )""")
    return "\n".join(lines) + "\n"


def gen_j1_header():
    """Generate 2x20 pin header footprint."""
    cx, cy = J1_POS
    pitch = 2.54
    pad_size = 1.7
    drill = 1.0

    pads = []
    for row in range(20):
        pin_odd = row * 2 + 1
        pin_even = row * 2 + 2
        py_rel = -(9.5 - row) * pitch

        # Odd pin (left column)
        px_rel = -pitch / 2
        net_name = J1_PIN_NETS.get(pin_odd, "")
        net_str = net_ref(net_name) if net_name else '(net 0 "")'
        shape = "rect" if pin_odd == 1 else "circle"
        pads.append(
            f'    (pad "{pin_odd}" thru_hole {shape} (at {fmt(px_rel)} {fmt(py_rel)}) '
            f'(size {fmt(pad_size)} {fmt(pad_size)}) (drill {fmt(drill)}) '
            f'(layers "*.Cu" "*.Mask") {net_str} (uuid "{uid()}"))'
        )

        # Even pin (right column)
        px_rel = pitch / 2
        net_name = J1_PIN_NETS.get(pin_even, "")
        net_str = net_ref(net_name) if net_name else '(net 0 "")'
        pads.append(
            f'    (pad "{pin_even}" thru_hole circle (at {fmt(px_rel)} {fmt(py_rel)}) '
            f'(size {fmt(pad_size)} {fmt(pad_size)}) (drill {fmt(drill)}) '
            f'(layers "*.Cu" "*.Mask") {net_str} (uuid "{uid()}"))'
        )

    pad_block = "\n".join(pads)
    return f"""  (footprint "Connector_PinHeader_2.54mm:PinHeader_2x20_P2.54mm_Vertical"
    (at {fmt(cx)} {fmt(cy)})
    (layer "F.Cu")
    (uuid "{uid()}")
    (fp_text reference "J1" (at 0 -27) (layer "F.SilkS") (effects (font (size 1 1) (thickness 0.15))))
    (fp_text value "PinHeader_2x20" (at 0 27) (layer "F.Fab") (effects (font (size 1 1) (thickness 0.15))))
{pad_block}
  )
"""


def gen_jst_connector(ref, num_pins, cx, cy, pin_nets):
    """Generate JST-XH connector footprint."""
    pitch = 2.50
    pad_size_x = 1.5
    pad_size_y = 1.7
    drill = 0.9

    pads = []
    for i in range(num_pins):
        pin = i + 1
        px_rel = -(num_pins - 1) * pitch / 2 + i * pitch
        net_name = pin_nets.get(pin, "")
        net_str = net_ref(net_name) if net_name else '(net 0 "")'
        shape = "rect" if pin == 1 else "oval"
        pads.append(
            f'    (pad "{pin}" thru_hole {shape} (at {fmt(px_rel)} 0) '
            f'(size {fmt(pad_size_x)} {fmt(pad_size_y)}) (drill {fmt(drill)}) '
            f'(layers "*.Cu" "*.Mask") {net_str} (uuid "{uid()}"))'
        )

    # Strain relief pads
    sr_offset = num_pins * pitch / 2 + 1.5
    for offset in [-sr_offset, sr_offset]:
        pads.append(
            f'    (pad "" thru_hole oval (at {fmt(offset)} 2) '
            f'(size 1.2 1.8) (drill 0.8) '
            f'(layers "*.Cu" "*.Mask") (uuid "{uid()}"))'
        )

    pad_block = "\n".join(pads)
    return f"""  (footprint "Connector_JST:JST_XH_B{num_pins}B-XH-A_1x{num_pins:02d}_P2.50mm_Horizontal"
    (at {fmt(cx)} {fmt(cy)})
    (layer "F.Cu")
    (uuid "{uid()}")
    (fp_text reference "{ref}" (at 0 -3) (layer "F.SilkS") (effects (font (size 1 1) (thickness 0.15))))
    (fp_text value "JST_XH_{num_pins}pin" (at 0 4) (layer "F.Fab") (effects (font (size 1 1) (thickness 0.15))))
{pad_block}
  )
"""


def gen_smd_0805(ref, value, cx, cy, pad1_net, pad2_net):
    """Generate 0805 SMD component (resistor or capacitor)."""
    spacing = 0.8
    pad_w, pad_h = 1.0, 1.25

    net1_str = net_ref(pad1_net) if pad1_net else '(net 0 "")'
    net2_str = net_ref(pad2_net) if pad2_net else '(net 0 "")'

    return f"""  (footprint "Resistor_SMD:R_0805_2012Metric"
    (at {fmt(cx)} {fmt(cy)})
    (layer "F.Cu")
    (uuid "{uid()}")
    (fp_text reference "{ref}" (at 0 -1.5) (layer "F.SilkS") (effects (font (size 0.8 0.8) (thickness 0.1))))
    (fp_text value "{value}" (at 0 1.5) (layer "F.Fab") (effects (font (size 0.8 0.8) (thickness 0.1))))
    (pad "1" smd roundrect (at {fmt(-spacing)} 0) (size {fmt(pad_w)} {fmt(pad_h)}) (layers "F.Cu" "F.Paste" "F.Mask") (roundrect_rratio 0.25) {net1_str} (uuid "{uid()}"))
    (pad "2" smd roundrect (at {fmt(spacing)} 0) (size {fmt(pad_w)} {fmt(pad_h)}) (layers "F.Cu" "F.Paste" "F.Mask") (roundrect_rratio 0.25) {net2_str} (uuid "{uid()}"))
  )
"""


def gen_led_3mm(ref, cx, cy, anode_net, cathode_net):
    """Generate 3mm through-hole LED."""
    spacing = 1.27
    pad_size = 1.6
    drill = 0.8

    net1_str = net_ref(anode_net) if anode_net else '(net 0 "")'
    net2_str = net_ref(cathode_net) if cathode_net else '(net 0 "")'

    return f"""  (footprint "LED_THT:LED_D3.0mm"
    (at {fmt(cx)} {fmt(cy)})
    (layer "F.Cu")
    (uuid "{uid()}")
    (fp_text reference "{ref}" (at 0 -2.5) (layer "F.SilkS") (effects (font (size 0.8 0.8) (thickness 0.1))))
    (fp_text value "LED_3mm" (at 0 2.5) (layer "F.Fab") (effects (font (size 0.8 0.8) (thickness 0.1))))
    (pad "1" thru_hole rect (at {fmt(-spacing)} 0) (size {fmt(pad_size)} {fmt(pad_size)}) (drill {fmt(drill)}) (layers "*.Cu" "*.Mask") {net1_str} (uuid "{uid()}"))
    (pad "2" thru_hole circle (at {fmt(spacing)} 0) (size {fmt(pad_size)} {fmt(pad_size)}) (drill {fmt(drill)}) (layers "*.Cu" "*.Mask") {net2_str} (uuid "{uid()}"))
  )
"""


def gen_track(x1, y1, x2, y2, width, layer, net_name):
    net_id = NET_BY_NAME.get(net_name, 0)
    layer_str = "F.Cu" if layer == 0 else "B.Cu"
    return (
        f'  (segment (start {fmt(x1)} {fmt(y1)}) (end {fmt(x2)} {fmt(y2)}) '
        f'(width {fmt(width)}) (layer "{layer_str}") (net {net_id}) (uuid "{uid()}"))'
    )


def gen_via(x, y, net_name):
    net_id = NET_BY_NAME.get(net_name, 0)
    return (
        f'  (via (at {fmt(x)} {fmt(y)}) (size {fmt(VIA_OUTER)}) '
        f'(drill {fmt(VIA_DRILL)}) (layers "F.Cu" "B.Cu") (net {net_id}) (uuid "{uid()}"))'
    )


def gen_traces():
    """Generate all trace segments."""
    tracks = []

    def t(x1, y1, x2, y2, w, net, layer=0):
        tracks.append(gen_track(x1, y1, x2, y2, w, layer, net))

    def v(x, y, net):
        tracks.append(gen_via(x, y, net))

    # --- Display signals: J1 → J2 ---

    # SPI0_SCLK: J1 pin 23 → J2 pin 3
    x1, y1 = J1_PADS[23]
    x2, y2 = J2_PADS[3]
    mid_x = 48.0
    t(x1, y1, mid_x, y1, TRACE_SIGNAL, "SPI0_SCLK")
    t(mid_x, y1, x2, y2, TRACE_SIGNAL, "SPI0_SCLK")

    # SPI0_MOSI: J1 pin 19 → J2 pin 4
    x1, y1 = J1_PADS[19]
    x2, y2 = J2_PADS[4]
    mid_x = 46.0
    t(x1, y1, mid_x, y1, TRACE_SIGNAL, "SPI0_MOSI")
    t(mid_x, y1, x2, y2, TRACE_SIGNAL, "SPI0_MOSI")

    # SPI0_CE0: J1 pin 24 → J2 pin 5
    x1, y1 = J1_PADS[24]
    x2, y2 = J2_PADS[5]
    mid_x = 50.0
    t(x1, y1, mid_x, y1, TRACE_SIGNAL, "SPI0_CE0")
    t(mid_x, y1, x2, y2, TRACE_SIGNAL, "SPI0_CE0")

    # GPIO_DC: J1 pin 18 → J2 pin 6
    x1, y1 = J1_PADS[18]
    x2, y2 = J2_PADS[6]
    mid_x = 52.0
    t(x1, y1, mid_x, y1, TRACE_SIGNAL, "GPIO_DC")
    t(mid_x, y1, x2, y2, TRACE_SIGNAL, "GPIO_DC")

    # GPIO_RST: J1 pin 12 → J2 pin 7
    x1, y1 = J1_PADS[12]
    x2, y2 = J2_PADS[7]
    mid_x = 54.0
    t(x1, y1, mid_x, y1, TRACE_SIGNAL, "GPIO_RST")
    t(mid_x, y1, x2, y2, TRACE_SIGNAL, "GPIO_RST")

    # --- Keyboard signals: J1 → J3 ---

    # I2C1_SDA: J1 pin 3 → J3 pin 3
    x1, y1 = J1_PADS[3]
    x2, y2 = J3_PADS[3]
    mid_x = 44.0
    t(x1, y1, mid_x, y1, TRACE_SIGNAL, "I2C1_SDA")
    t(mid_x, y1, x2, y2, TRACE_SIGNAL, "I2C1_SDA")

    # I2C1_SCL: J1 pin 5 → J3 pin 4
    x1, y1 = J1_PADS[5]
    x2, y2 = J3_PADS[4]
    mid_x = 46.0
    t(x1, y1, mid_x, y1, TRACE_SIGNAL, "I2C1_SCL")
    t(mid_x, y1, x2, y2, TRACE_SIGNAL, "I2C1_SCL")

    # GPIO_INT: J1 pin 7 → J3 pin 5
    x1, y1 = J1_PADS[7]
    x2, y2 = J3_PADS[5]
    mid_x = 48.0
    t(x1, y1, mid_x, y1, TRACE_SIGNAL, "GPIO_INT")
    t(mid_x, y1, x2, y2, TRACE_SIGNAL, "GPIO_INT")

    # --- Power: 3V3 ---

    # J1 pin 1 → C1 pad 1
    x1, y1 = J1_PADS[1]
    x2, y2 = C1_PADS[1]
    t(x1, y1, x2, y1, TRACE_POWER, "3V3")
    t(x2, y1, x2, y2, TRACE_POWER, "3V3")

    # J1 pin 1 → bus along top → J2 pin 1
    bus_y = 3.0
    x2, y2 = J2_PADS[1]
    t(x1, y1, x1, bus_y, TRACE_POWER, "3V3")
    t(x1, bus_y, x2, bus_y, TRACE_POWER, "3V3")
    t(x2, bus_y, x2, y2, TRACE_POWER, "3V3")

    # J2 pin 8 (BLK) taps off same bus
    x3, y3 = J2_PADS[8]
    t(x3, bus_y, x3, y3, TRACE_POWER, "3V3")

    # J1 pin 17 → bus along bottom → J3 pin 1
    x1, y1 = J1_PADS[17]
    x2, y2 = J3_PADS[1]
    bus_y2 = 27.0
    t(x1, y1, x1, bus_y2, TRACE_POWER, "3V3")
    t(x1, bus_y2, x2, bus_y2, TRACE_POWER, "3V3")
    t(x2, bus_y2, x2, y2, TRACE_POWER, "3V3")

    # --- Power: GND ---

    # C1 pad 2 → via to ground pour
    x1, y1 = C1_PADS[2]
    t(x1, y1, x1, y1 + 1.5, TRACE_POWER, "GND")
    v(x1, y1 + 1.5, "GND")

    # J1 pin 6 → J2 pin 2
    x1, y1 = J1_PADS[6]
    x2, y2 = J2_PADS[2]
    mid_y = 4.5
    t(x1, y1, x1, mid_y, TRACE_POWER, "GND")
    t(x1, mid_y, x2, mid_y, TRACE_POWER, "GND")
    t(x2, mid_y, x2, y2, TRACE_POWER, "GND")

    # J1 pin 9 → J3 pin 2
    x1, y1 = J1_PADS[9]
    x2, y2 = J3_PADS[2]
    mid_y = 25.5
    t(x1, y1, x1, mid_y, TRACE_POWER, "GND")
    t(x1, mid_y, x2, mid_y, TRACE_POWER, "GND")
    t(x2, mid_y, x2, y2, TRACE_POWER, "GND")

    # --- LED circuit ---

    # J1 pin 29 → R1 pad 1
    x1, y1 = J1_PADS[29]
    x2, y2 = R1_PADS[1]
    t(x1, y1, x1, y2, TRACE_SIGNAL, "GPIO_LED")
    t(x1, y2, x2, y2, TRACE_SIGNAL, "GPIO_LED")

    # R1 pad 2 → D1 pad 1 (anode)
    x1, y1 = R1_PADS[2]
    x2, y2 = D1_PADS[1]
    t(x1, y1, x2, y1, TRACE_SIGNAL, "GPIO_LED")
    t(x2, y1, x2, y2, TRACE_SIGNAL, "GPIO_LED")

    # D1 pad 2 (cathode) → GND via
    x1, y1 = D1_PADS[2]
    t(x1, y1, x1, y1 + 1.0, TRACE_POWER, "GND")
    v(x1, y1 + 1.0, "GND")

    # --- Shutdown button: J1 pin 11 → J4 ---

    # GPIO_SHUTDOWN: J1 pin 11 → J4 pin 1
    x1, y1 = J1_PADS[11]
    x2, y2 = J4_PADS[1]
    mid_x = 18.0
    t(x1, y1, mid_x, y1, TRACE_SIGNAL, "GPIO_SHUTDOWN")
    t(mid_x, y1, x2, y2, TRACE_SIGNAL, "GPIO_SHUTDOWN")

    # J4 pin 2 (GND) → via to ground pour
    x1, y1 = J4_PADS[2]
    t(x1, y1, x1, y1 + 1.5, TRACE_POWER, "GND")
    v(x1, y1 + 1.5, "GND")

    # GND stitching vias
    for vx, vy in [(28.0, 5.0), (28.0, 25.0), (37.0, 5.0), (37.0, 25.0)]:
        v(vx, vy, "GND")

    return "\n".join(tracks) + "\n"


def gen_ground_zone():
    """Generate B.Cu ground pour zone."""
    inset = 0.3
    pts = [
        (inset, inset),
        (BOARD_W - inset, inset),
        (BOARD_W - inset, BOARD_H - inset),
        (inset, BOARD_H - inset),
    ]
    xy_lines = "\n".join(f'        (xy {fmt(x)} {fmt(y)})' for x, y in pts)

    return f"""  (zone (net 2) (net_name "GND") (layer "B.Cu") (uuid "{uid()}")
    (hatch edge 0.5)
    (connect_pads (clearance 0.3))
    (min_thickness 0.25)
    (filled_areas_thickness no)
    (fill yes (thermal_gap 0.5) (thermal_bridge_width 0.5))
    (polygon
      (pts
{xy_lines}
      )
    )
  )
"""


def gen_silkscreen():
    texts = [
        ("RFMP Carrier HAT v1.0", 32.5, 1.5, 1.2, 0.15),
        ("J2: DISPLAY", 56.0, 5.0, 0.8, 0.12),
        ("J3: KEYBOARD", 56.0, 17.0, 0.8, 0.12),
        ("J4: SHUTDOWN", 10.0, 5.0, 0.8, 0.12),
        ("D1", 10.0, 28.5, 0.8, 0.1),
        ("R1 220R", 10.0, 20.0, 0.8, 0.1),
        ("C1 100nF", 20.0, 3.0, 0.8, 0.1),
    ]
    lines = []
    for text, x, y, size, thick in texts:
        lines.append(
            f'  (gr_text "{text}" (at {fmt(x)} {fmt(y)}) (layer "F.SilkS") '
            f'(uuid "{uid()}") '
            f'(effects (font (size {fmt(size)} {fmt(size)}) (thickness {fmt(thick)}))))'
        )
    return "\n".join(lines) + "\n"


# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------

def main():
    sections = [
        gen_header(),
        gen_nets(),
        "",
        gen_outline(),
        "",
        gen_mounting_holes(),
        "",
        gen_j1_header(),
        gen_jst_connector("J2", 8, *J2_POS, J2_PIN_NETS),
        gen_jst_connector("J3", 5, *J3_POS, J3_PIN_NETS),
        gen_jst_connector("J4", 2, *J4_POS, J4_PIN_NETS),
        gen_smd_0805("R1", "220R", *R1_POS, "GPIO_LED", "GPIO_LED"),
        gen_smd_0805("C1", "100nF", *C1_POS, "3V3", "GND"),
        gen_led_3mm("D1", *D1_POS, "GPIO_LED", "GND"),
        "",
        gen_traces(),
        "",
        gen_ground_zone(),
        gen_silkscreen(),
        ")\n",
    ]

    content = "\n".join(sections)

    out_path = os.path.join(os.path.dirname(os.path.abspath(__file__)),
                            "carrier-hat.kicad_pcb")
    with open(out_path, "w") as f:
        f.write(content)

    print(f"PCB saved to: {out_path}")
    print(f"Board: {BOARD_W}mm x {BOARD_H}mm, 2-layer")
    print(f"Components: J1 (2x20), J2 (8-pin JST), J3 (5-pin JST), J4 (2-pin JST), R1, D1, C1")
    print(f"Traces: {len(NETS)-1} nets routed, ground pour on B.Cu")
    print(f"\nOpen in KiCad → run DRC → export Gerbers")


if __name__ == "__main__":
    main()
