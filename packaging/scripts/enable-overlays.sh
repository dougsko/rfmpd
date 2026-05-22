#!/bin/sh
# Enable SPI and I2C device tree overlays for rfmp-display hardware backend.
# Safe to run multiple times (idempotent). No-op on boards without known config.

changed=0

# Orange Pi (Armbian)
if [ -f /boot/orangepiEnv.txt ]; then
    if grep -q "^overlays=" /boot/orangepiEnv.txt; then
        if ! grep "^overlays=" /boot/orangepiEnv.txt | grep -q "spi0-cs0"; then
            sed -i 's/^overlays=\(.*\)/overlays=\1 spi0-cs0/' /boot/orangepiEnv.txt
            changed=1
        fi
        if ! grep "^overlays=" /boot/orangepiEnv.txt | grep -q "i2c1"; then
            sed -i 's/^overlays=\(.*\)/overlays=\1 i2c1/' /boot/orangepiEnv.txt
            changed=1
        fi
    else
        echo "overlays=spi0-cs0 i2c1" >> /boot/orangepiEnv.txt
        changed=1
    fi
fi

# Raspberry Pi
if [ -f /boot/config.txt ]; then
    if ! grep -q "^dtparam=spi=on" /boot/config.txt; then
        echo "dtparam=spi=on" >> /boot/config.txt
        changed=1
    fi
    if ! grep -q "^dtparam=i2c_arm=on" /boot/config.txt; then
        echo "dtparam=i2c_arm=on" >> /boot/config.txt
        changed=1
    fi
fi

if [ "$changed" = "1" ]; then
    echo "NOTE: SPI/I2C overlays enabled. Reboot required for changes to take effect."
fi
