# RFMP Daemon Deployment

This document describes how rfmpd is installed, how it integrates with Direwolf TNC
via systemd and udev, and how the full hardware lifecycle works.

---

## 1. System Overview

```
┌──────────────────────────────────────────────────────────────────────┐
│                    Linux host (Orange Pi Zero 2W, RPi, etc.)           │
│                                                                       │
│  ┌─────────────┐       TCP :8001        ┌──────────────────────┐     │
│  │  Direwolf   │◄─────── KISS ─────────►│       rfmpd          │     │
│  │  (TNC)      │                         │  (RFMP daemon)       │     │
│  └──────┬──────┘                         └──────────┬───────────┘     │
│         │ audio + PTT                               │ HTTP :8080      │
│  ┌──────┴──────┐                         ┌──────────┴───────────┐     │
│  │  Sound card │                         │   Web UI / API       │     │
│  │  (Digirig)  │                         │   (browser client)   │     │
│  └──────┬──────┘                         └──────────────────────┘     │
│         │                                                             │
└─────────┼─────────────────────────────────────────────────────────────┘
          │ RF
       ┌──┴──┐
       │Radio│
       └─────┘
```

**rfmpd** connects to Direwolf over TCP KISS (default `127.0.0.1:8001`).
Direwolf handles the sound card, modem, and PTT control. rfmpd handles
protocol logic, storage, sync, and the web interface.

---

## 2. File Locations (after package install)

Packages install system-wide and create a dedicated `rfmpd` system user
(member of `audio`, `dialout`, `plugdev`).

| Path | Contents |
|------|----------|
| `/usr/bin/rfmpd` | Daemon binary |
| `/etc/rfmpd/config.yaml` | Daemon configuration |
| `/etc/rfmpd/direwolf/` | Direwolf config files (one per hardware profile) |
| `/var/lib/rfmpd/messages.db` | SQLite message database |
| `/var/log/rfmpd/rfmpd.log` | Log file |
| `/lib/systemd/system/rfmpd.service` | Daemon systemd unit |
| `/lib/systemd/system/direwolf@.service` | Direwolf template unit |
| `/usr/lib/udev/rules.d/99-rfmp-radio.rules` | USB device detection rules |

On Alpine, OpenRC service scripts are installed under `/etc/init.d/` and mdev
rules under `/etc/mdev.conf.d/` instead of systemd + udev.

---

## 3. Hardware Profiles

Each supported radio interface has a Direwolf config and a udev rule:

| Device | USB ID | Subsystem | Symlink | Instance | Config file |
|--------|--------|-----------|---------|----------|-------------|
| Digirig | `10c4:ea60` (CP210x) | tty | `/dev/digirig` | `digirig` | `direwolf-digirig.conf` |
| Digirig Lite | `0d8c:0012` (CM108) | hidraw | `/dev/digiriglite` | `digiriglite` | `direwolf-digiriglite.conf` |
| QMX | `0483:a34c` (STM32) | tty | `/dev/qmx` | `qmx` | `direwolf-qmx.conf` |

Instance names contain no hyphens to avoid systemd device path escaping issues.

---

## 4. USB Plug/Unplug Lifecycle

### Plug in

```
1. USB device appears
2. udev matches vendor:product ID in 99-rfmp-radio.rules
3. udev creates stable symlink (e.g. /dev/digirig)
4. udev sets SYSTEMD_WANTS=direwolf@digirig.service
5. systemd starts direwolf@digirig.service
6. Direwolf opens sound card, starts KISS TCP on port 8001
7. rfmpd (already running) connects to KISS port
8. Radio communication begins
```

### Unplug

```
1. USB device removed
2. Kernel removes /dev/digirig device node
3. systemd sees dev-digirig.device disappear
4. BindsTo=dev-digirig.device triggers service stop
5. Direwolf process terminated
6. rfmpd loses TCP connection, enters reconnect loop
7. (waits for next plug event)
```

### Crash recovery

If Direwolf crashes while the device is still present:
- `Restart=on-failure` in the service unit restarts it after 5 seconds
- rfmpd reconnects automatically via its reconnect loop

---

## 5. systemd Units

### rfmpd.service

```ini
[Unit]
Description=RFMP Daemon
After=network.target

[Service]
Type=simple
User=rfmpd
Group=rfmpd
ExecStart=/usr/bin/rfmpd -c /etc/rfmpd/config.yaml
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
```

Runs as the `rfmpd` system user. Runs independently of Direwolf — operates
in offline mode when no TNC is connected.

### direwolf@.service (template)

```ini
[Unit]
Description=Direwolf TNC (%i)
BindsTo=dev-%i.device
After=dev-%i.device sound.target

[Service]
Type=simple
User=rfmpd
Group=rfmpd
SupplementaryGroups=audio dialout plugdev
ExecStart=/usr/bin/direwolf -t 0 -c /etc/rfmpd/direwolf/direwolf-%i.conf
Restart=on-failure
RestartSec=5

[Install]
WantedBy=multi-user.target
```

The `%i` parameter is the instance name from udev (e.g. `digirig`, `qmx`).

`BindsTo=dev-%i.device` is the key mechanism: it creates a hard dependency
on the device node existing. When the device disappears (unplug), systemd
stops the service immediately.

Direwolf's KISS TCP server is enabled by default on port 8001 (no flag needed).
The `-t 0` flag disables terminal color codes for clean journald output.

### Alpine / OpenRC

Alpine packages install OpenRC service scripts at `/etc/init.d/rfmpd` and
`/etc/init.d/direwolf` and an mdev rule at `/etc/mdev.conf.d/rfmp-radio.conf`
in place of systemd + udev. The hot-plug flow is otherwise the same: mdev
matches the device on plug, direwolf starts, rfmpd reconnects.

---

## 6. udev Rules

```
# /usr/lib/udev/rules.d/99-rfmp-radio.rules

ACTION=="add", SUBSYSTEM=="tty", ATTRS{idVendor}=="10c4", ATTRS{idProduct}=="ea60", \
    SYMLINK+="digirig", TAG+="systemd", ENV{SYSTEMD_WANTS}+="direwolf@digirig.service"
```

Key fields:
- `ACTION=="add"` — only triggers on device insertion
- `SUBSYSTEM=="tty"` (or `hidraw` for the Digirig Lite CM108) — matches the
  device class
- `ATTRS{idVendor}/ATTRS{idProduct}` — USB vendor:product ID
- `SYMLINK+="digirig"` — creates `/dev/digirig` pointing to the actual node
- `TAG+="systemd"` — tells systemd to track this device
- `ENV{SYSTEMD_WANTS}` — tells systemd to start this system service

---

## 7. Direwolf Configuration

Each config file sets:
- `ADEVICE` — ALSA audio device for the sound card
- `MYCALL` — Callsign (substituted during install)
- `MODEM` — Modem speed (1200 baud for VHF, 300 for HF)
- `PTT` — Push-to-talk control method (varies by hardware)
- `FX25TX` — Forward error correction

Direwolf's KISS TCP server is enabled by default on port 8001 (no flag needed).
The `-t 0` flag disables terminal color codes for clean journald output.
rfmpd connects to this port as configured in `config.yaml`:

```yaml
network:
  direwolf_host: "127.0.0.1"
  direwolf_port: 8001
```

---

## 8. Adding a New Hardware Profile

To add support for a new radio interface:

1. Create `radio/direwolf-<name>.conf` with the appropriate audio device,
   modem speed, and PTT settings. Use no hyphens in `<name>`.

2. Add a udev rule to `radio/99-rfmp-radio.rules`:
   ```
   ACTION=="add", SUBSYSTEM=="tty", ATTRS{idVendor}=="XXXX", ATTRS{idProduct}=="YYYY", \
       SYMLINK+="<name>", TAG+="systemd", ENV{SYSTEMD_WANTS}+="direwolf@<name>.service"
   ```

3. The Dockerfile copies all `radio/direwolf-*.conf` files into the package,
   so a new config is picked up automatically — no further wiring needed.

4. Find the USB vendor:product ID with `lsusb` or `udevadm info /dev/ttyUSBx`.

---

## 9. Manual Operations

```bash
# Check rfmpd status
sudo systemctl status rfmpd

# Check Direwolf status (replace 'digirig' with your device)
sudo systemctl status direwolf@digirig

# View rfmpd logs
sudo journalctl -u rfmpd -f

# View Direwolf logs
sudo journalctl -u direwolf@digirig -f

# Restart rfmpd
sudo systemctl restart rfmpd

# Manually start Direwolf (without udev)
sudo systemctl start direwolf@digirig

# Reload udev rules after editing
sudo udevadm control --reload-rules

# Test udev rule matching
udevadm test /sys/class/tty/ttyUSB0
```

---

## 10. Troubleshooting

| Symptom | Cause | Fix |
|---------|-------|-----|
| rfmpd says "connecting to Direwolf..." forever | Direwolf not running or wrong port | Check `sudo systemctl status 'direwolf@*'` |
| Direwolf doesn't start on USB plug | udev rule not loaded | `sudo udevadm control --reload-rules` then re-plug |
| Direwolf starts but no audio | Wrong ALSA device in config | Check `aplay -l` for device name, update conf |
| Direwolf stops immediately after start | Device symlink doesn't exist | Check udev rule matches your USB IDs |
| "Permission denied" on /dev/ttyUSB* | User not in dialout group | `sudo usermod -aG dialout $USER` then re-login |
| Service doesn't stop on unplug | BindsTo not matching device | Verify symlink name matches instance name (no hyphens) |
