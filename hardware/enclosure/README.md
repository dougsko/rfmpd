# RFMP Pager Enclosure

Parametric 3D-printable enclosure for the RFMP pager hardware build.

## Requirements

- [OpenSCAD](https://openscad.org/downloads.html) (free, macOS/Windows/Linux)
  - macOS: `brew install openscad`

## Usage

1. Open `pager-enclosure.scad` in OpenSCAD
2. Adjust parameters in the Customizer panel (right sidebar) if needed
3. Change the `part` variable to export each piece:
   - `"assembly"` — preview all parts together (default)
   - `"box"` — bottom enclosure half (for STL export)
   - `"lid"` — top enclosure half (for STL export)
4. **F5** to preview, **F6** to render (slow but accurate)
5. File → Export → STL for each part

## Printing

Upload the two STL files (box + lid) to a 3D printing service:

| Service | Material | Notes |
|---------|----------|-------|
| JLCPCB 3D Printing | SLA resin (black) | ~$15-20, order alongside carrier HAT PCB |
| PCBWay 3D Printing | MJF Nylon (PA12) | Stronger, good for enclosures |
| Local FDM printer | PLA or PETG | 0.2mm layer height, 3 perimeters, 20% infill |

## Assembly

1. Insert 4× M3 heat-set inserts into screw boss holes in the bottom box
   (or use self-tapping M3 screws if printing in plastic)
2. Mount OPi Zero 2W to bottom standoffs with M2.5×5mm screws
3. Plug carrier HAT onto OPi 40-pin header (HAT sits on same standoffs)
4. Slide LiPo battery into the open-ended pocket (JST connector for quick disconnect)
5. Seat IP5306 module in its pocket, USB-C port aligned with side cutout
6. Mount display and keyboard to lid underside posts with M2×4mm screws
7. Route JST-XH cables from display/keyboard through to carrier HAT connectors
8. Close lid, secure with 4× M3×12mm screws through countersunk holes

## Design Notes

- Battery compartment is open on one end — slide the LiPo out without
  disassembling anything. A retaining lip on the back end keeps it from
  sliding out during use.
- All dimensions are parametric — edit the variables at the top of the
  .scad file to accommodate different component sizes.
- The alignment lip on the box top edge keys into the lid for a snug fit.
- Corner screw bosses are countersunk on the lid side for flush M3 heads.

## File

```
hardware/enclosure/
├── README.md                  # This file
└── pager-enclosure.scad       # OpenSCAD parametric enclosure
```
