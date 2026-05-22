# Manual Installation

For systems without .deb/.rpm/.apk package support.

## Install

```sh
# Copy binary
sudo cp rfmpd /usr/bin/rfmpd
sudo chmod 755 /usr/bin/rfmpd

# Create system user
sudo useradd --system --home-dir /var/lib/rfmpd --shell /usr/sbin/nologin rfmpd
sudo usermod -a -G audio,dialout,plugdev rfmpd

# Install config
sudo mkdir -p /etc/rfmpd/direwolf
sudo cp config.yaml /etc/rfmpd/config.yaml
sudo cp direwolf/*.conf /etc/rfmpd/direwolf/

# Create data/log directories
sudo mkdir -p /var/lib/rfmpd /var/log/rfmpd
sudo chown rfmpd:rfmpd /var/lib/rfmpd /var/log/rfmpd

# Install systemd units (if using systemd)
sudo cp systemd/rfmpd.service /lib/systemd/system/
sudo cp systemd/direwolf@.service /lib/systemd/system/
sudo systemctl daemon-reload

# Install udev rules
sudo cp udev/99-rfmp-radio.rules /usr/lib/udev/rules.d/
sudo udevadm control --reload-rules

# Enable and start
sudo systemctl enable --now rfmpd

# Enable SPI/I2C overlays for display hardware (reboot required)
sudo /usr/share/rfmpd/enable-overlays.sh
```

### Display hardware setup

The `rfmp-display` hardware backend requires SPI and I2C to be enabled.
The package post-install script handles this automatically, but if you need
to do it manually:

**Orange Pi (Armbian)** — edit `/boot/orangepiEnv.txt`:
```
overlays=spi0-cs0 i2c1
```

**Raspberry Pi** — edit `/boot/config.txt`:
```
dtparam=spi=on
dtparam=i2c_arm=on
```

Reboot after making changes. Verify with:
```sh
ls /dev/spidev0.0   # display
ls /dev/i2c-1       # keyboard
```

## Uninstall

```sh
sudo systemctl disable --now rfmpd
sudo rm /usr/bin/rfmpd
sudo rm -rf /etc/rfmpd
sudo rm /lib/systemd/system/rfmpd.service /lib/systemd/system/direwolf@.service
sudo rm /usr/lib/udev/rules.d/99-rfmp-radio.rules
sudo userdel rfmpd
sudo rm -rf /var/lib/rfmpd /var/log/rfmpd
sudo systemctl daemon-reload
```
