# RFMP Daemon (Go)

RF Microblog Protocol (RFMP) v0.5 daemon for packet radio networks.

## Overview

RFMP is a decentralized, append-only microblogging protocol for AX.25 UI frame packet radio networks. This daemon provides:

- Full RFMP protocol (Protocol Buffers wire format)
- Scuttlebutt-style vector clock synchronization
- REST API and real-time WebSocket updates
- Integrated web UI on port 8080
- SQLite persistent storage (pure Go, no CGo)
- Automatic Direwolf integration via udev
- Single static binary (~15MB), cross-compiles to ARM
- Native packages: `.deb`, `.rpm`, `.apk`, plus tarball

## Prerequisites

- [Go 1.24+](https://go.dev/dl/) (build only)
- Linux (Debian-based) for deployment
- [Direwolf](https://github.com/wb2osz/direwolf) TNC
- Amateur radio license (for transmission)
- Supported radio interface: Digirig, Digirig Lite, or QRP Labs QMX

## Building

```bash
# Build for current platform (stripped, version info baked in)
make build

# Plain build for current platform
go build -o rfmpd .

# Cross-compile for Raspberry Pi (ARM64)
GOOS=linux GOARCH=arm64 go build -o rfmpd .

# Cross-compile for Raspberry Pi (ARMv7) / Orange Pi Zero
GOOS=linux GOARCH=arm GOARM=7 go build -o rfmpd .

# Build all native packages (.deb, .rpm, .apk) for arm64 + amd64 (requires Docker)
make dist
```

`make build` uses `-ldflags "-s -w -X main.version=…"` to strip debug symbols
(reducing binary size ~30%) and embed the version string from the Makefile.

No CGo is required — the SQLite driver (`modernc.org/sqlite`) is pure Go, so cross-compilation works without a C toolchain.

## Running Tests

```bash
# Run unit tests
go test ./...

# Run protocol simulation (end-to-end)
go build -o rfmpd . && go build -o rf-sim ./cmd/rf-sim/
./rf-sim                        # All scenarios, unlimited speed
./rf-sim --baud 1200            # Simulate 1200 baud VHF channel
./rf-sim --baud 300             # Simulate 300 baud HF channel
./rf-sim --verbose              # Show broker frame traffic
./rf-sim --continuous           # Continuous churn testing
```

The simulation spawns multiple rfmpd nodes connected through an in-process
KISS broker, then verifies message delivery under various conditions:
network partitions, node crashes, late joiners, and fragmentation.

## Quick Start

```bash
# Build for current platform
make build

# Run in offline mode (no Direwolf needed)
./rfmpd -c config.yaml.example

# Run with verbose logging
./rfmpd -c config.yaml.example -v

# Open the web UI
open http://localhost:8080
```

## Installation (Linux)

Install from a native package — the `.deb`, `.rpm`, and `.apk` packages bundle
the binary, default config, systemd/OpenRC unit files, Direwolf configs, and
udev rules. Build them with `make dist` (requires Docker):

```bash
make dist             # builds .deb, .rpm, .apk, and tarballs in ./dist/

# Debian/Ubuntu/Raspberry Pi OS
sudo apt install ./rfmpd_0.5.0_arm64.deb

# Fedora/RHEL
sudo dnf install ./rfmpd-0.5.0-1.aarch64.rpm

# Alpine
sudo apk add --allow-untrusted ./rfmpd-0.5.0-1-aarch64.apk
```

After install, set your callsign and start the service:

```bash
sudo nano /etc/rfmpd/config.yaml                       # set callsign
sudo nano /etc/rfmpd/direwolf/direwolf-digirig.conf    # set MYCALL to match
sudo systemctl enable --now rfmpd
```

Direwolf starts automatically when a supported radio interface is plugged in
via USB. For systems without package support, see
[packaging/INSTALL.md](packaging/INSTALL.md) for the manual tarball flow.
See [DEPLOYMENT.md](DEPLOYMENT.md) for full details on the hardware lifecycle.

## Uninstalling

```bash
# Debian/Ubuntu — keep config and data
sudo apt remove rfmpd
# Debian/Ubuntu — remove everything including config
sudo apt purge rfmpd

# Fedora/RHEL
sudo dnf remove rfmpd

# Alpine
sudo apk del rfmpd
```

## Supported Hardware

| Interface | Band | Baud | PTT | USB ID |
|-----------|------|------|-----|--------|
| Digirig | VHF | 1200 | Serial RTS | `10c4:ea60` |
| Digirig Lite | VHF | 1200 | CM108 GPIO | `0d8c:0012` (hidraw) |
| QMX | HF | 300 | Hamlib CAT | `0483:a34c` |

The packages install udev rules that auto-detect these devices and start Direwolf with the correct configuration.

## Configuration

Config file: `/etc/rfmpd/config.yaml` (installed) or `./config.yaml` (dev)

```yaml
node:
  callsign: "N0CALL"    # Your callsign (REQUIRED)
  ssid: 0               # SSID 0-15

network:
  direwolf_host: "127.0.0.1"
  direwolf_port: 8001
  offline_mode: false    # true for testing without radio

sync:
  sync_interval: 60     # SVEC broadcast interval (seconds)

storage:
  database_path: "/var/lib/rfmpd/messages.db"

api:
  host: "0.0.0.0"
  port: 8080
  cors_origins:
    - "http://localhost:8080"
```

## API

The daemon serves a REST API on port 8080 (configurable):

| Endpoint | Method | Description |
|----------|--------|-------------|
| `/` | GET | Web UI |
| `/health` | GET | Health check |
| `/messages` | GET | List messages (filterable) |
| `/messages/{id}` | GET | Get specific message |
| `/messages` | POST | Send a message |
| `/channels` | GET | List active channels |
| `/nodes` | GET | List seen nodes |
| `/status` | GET | Daemon status and vector clock |
| `/config/callsign` | GET/POST | Get or set callsign |
| `/stream` | WS | Real-time WebSocket updates |

```bash
# Send a message
curl -X POST http://localhost:8080/messages \
  -H "Content-Type: application/json" \
  -d '{"channel": "general", "body": "Hello RFMP!"}'

# Get recent messages
curl http://localhost:8080/messages?channel=general&limit=10

# Check status
curl http://localhost:8080/status
```

See [openapi.yaml](openapi.yaml) for the full API specification.

## Protocol

RFMP v0.4 uses Protocol Buffers for the RF wire format:

- **MSG** — Message with per-node monotonic sequence number, epoch timestamp, binary ID
- **FRAG** — Fragment for messages exceeding threshold (raw protobuf bytes, no base64)
- **SVEC** — Vector clock broadcast for scuttlebutt synchronization (native map type)

Protobuf encoding saves 30-50% airtime versus the previous text format. Synchronization uses per-node sequence numbers. Nodes broadcast their vector clock via SVEC frames; on receiving a remote SVEC, the local node pushes any messages the remote is missing.

See [docs/PROTOCOL.md](docs/PROTOCOL.md) for the complete protocol specification and [test_vectors.json](test_vectors.json) for conformance vectors.

## Display Simulator

A standalone pager-style UI that renders to a 240×320 framebuffer in the browser. Targets the hardware pager build (ILI9341 display + BBQ20KBD keyboard on Orange Pi Zero 2W) but develops entirely on desktop via a web-based canvas simulator — no SDL or native GUI library required.

```bash
# Build the display simulator
go build -tags sim -o rfmp-display-sim ./cmd/rfmp-display

# Run (point at your running rfmpd instance)
./rfmp-display-sim -addr http://localhost:8080

# Open the simulator canvas in your browser
open http://localhost:9090
```

The simulator serves a web page at `localhost:9090` with a pixel-accurate 240×320 canvas (scaled 2× to 480×640). Click the canvas to focus, then interact with the keyboard:

| Key | Action |
|-----|--------|
| Arrow Up/Down | Navigate lists, scroll messages |
| Enter | Select channel, send message |
| Escape | Go back |
| Any letter | Open compose screen |
| `n` | Create new channel (from channel list) |
| `d` | Delete empty channel (from channel list) |
| Backspace | Delete character in compose |

The display connects to rfmpd via the same REST API and WebSocket as the web UI. Messages sent from either interface appear in both in real-time.

```bash
# Cross-compile for hardware (Orange Pi Zero 2W)
GOOS=linux GOARCH=arm64 go build -tags hw -o rfmp-display ./cmd/rfmp-display
```

See [hardware/pager-concept.md](hardware/pager-concept.md) for the full hardware build plan and [hardware/carrier-hat/](hardware/carrier-hat/) for the carrier PCB design.

## Project Structure

```
rfmp-go/
├── main.go                    # Entry point, embed web UI, signal handling
├── config.yaml.example        # Example configuration
├── Makefile                   # Build + multi-arch package builds (Docker)
├── Dockerfile                 # Multi-stage build for .deb/.rpm/.apk/tarball
├── cmd/
│   ├── rf-sim/                # Protocol simulation test harness
│   │   ├── main.go            # CLI, scenario runner
│   │   ├── broker.go          # KISS broker with partition/baud simulation
│   │   ├── node.go            # Node subprocess lifecycle
│   │   ├── api.go             # HTTP client helpers
│   │   ├── wsclient.go        # WebSocket client for assertions
│   │   ├── scenarios.go       # Test scenarios
│   │   └── report.go          # Results output
│   └── rfmp-display/          # Pager display UI + simulator
│       ├── main.go            # Main loop, signal handling
│       ├── api.go             # HTTP client for rfmpd
│       ├── ws.go              # WebSocket client with reconnect
│       ├── backend.go         # Display/Keyboard/LED interfaces
│       ├── backend_sim.go     # Web-based canvas simulator (build tag: sim)
│       ├── backend_hw.go      # Hardware stubs (build tag: hw)
│       ├── sim.html           # Browser canvas renderer
│       ├── framebuffer.go     # RGB565 pixel buffer + drawing primitives
│       ├── font.go            # Embedded 8×16 VGA bitmap font
│       ├── colors.go          # RGB565 color constants
│       ├── ui.go              # Screen state machine
│       ├── ui_channels.go     # Channel list screen
│       ├── ui_timeline.go     # Message timeline screen
│       ├── ui_compose.go      # Text compose screen
│       └── ui_create_channel.go # Channel name input screen
├── internal/
│   ├── config/                # YAML config loading with defaults
│   ├── protocol/              # Protobuf wire format, frames, fragmentation, message IDs
│   │   └── pb/                # Generated protobuf code (.proto schema)
│   ├── network/               # KISS framing, AX.25, Direwolf TCP client
│   ├── storage/               # SQLite database (pure Go)
│   ├── sync/                  # Adaptive transmission timing
│   ├── api/                   # HTTP server, WebSocket, route handlers
│   └── daemon/                # Main orchestrator (loops, routing, sync)
├── web/                       # Embedded web UI (HTML/CSS/JS)
├── radio/                     # Direwolf configs and udev rules
├── packaging/                 # Per-format package metadata (deb/rpm/apk/systemd/openrc/mdev) + INSTALL.md
├── hardware/                  # Pager hardware: concept, carrier HAT, 3D-printable enclosure
├── docs/                      # Protocol specification + RFC
├── test_vectors.json          # Protocol conformance test vectors
└── DEPLOYMENT.md              # Deployment and hardware lifecycle docs
```

## License

MIT License
