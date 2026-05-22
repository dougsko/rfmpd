Name:           %{pkg_name}
Version:        %{pkg_version}
Release:        1
Summary:        %{pkg_summary}
License:        %{pkg_license}
Requires:       %{pkg_depends}
Conflicts:      %{pkg_conflicts}

%description
%{pkg_summary}

%install
mkdir -p %{buildroot}/usr/bin
mkdir -p %{buildroot}/etc/rfmpd/direwolf
mkdir -p %{buildroot}/lib/systemd/system
mkdir -p %{buildroot}/usr/lib/udev/rules.d
mkdir -p %{buildroot}/usr/share/rfmpd
cp %{_sourcedir}/rfmpd %{buildroot}/usr/bin/rfmpd
cp %{_sourcedir}/config.yaml %{buildroot}/etc/rfmpd/config.yaml
cp %{_sourcedir}/direwolf-digirig.conf %{buildroot}/etc/rfmpd/direwolf/
cp %{_sourcedir}/direwolf-digiriglite.conf %{buildroot}/etc/rfmpd/direwolf/
cp %{_sourcedir}/direwolf-qmx.conf %{buildroot}/etc/rfmpd/direwolf/
cp %{_sourcedir}/rfmpd.service %{buildroot}/lib/systemd/system/
cp %{_sourcedir}/direwolf@.service %{buildroot}/lib/systemd/system/
cp %{_sourcedir}/99-rfmp-radio.rules %{buildroot}/usr/lib/udev/rules.d/
cp %{_sourcedir}/enable-overlays.sh %{buildroot}/usr/share/rfmpd/

%files
/usr/bin/rfmpd
%config(noreplace) /etc/rfmpd/config.yaml
%config(noreplace) /etc/rfmpd/direwolf/direwolf-digirig.conf
%config(noreplace) /etc/rfmpd/direwolf/direwolf-digiriglite.conf
%config(noreplace) /etc/rfmpd/direwolf/direwolf-qmx.conf
/lib/systemd/system/rfmpd.service
/lib/systemd/system/direwolf@.service
/usr/lib/udev/rules.d/99-rfmp-radio.rules
/usr/share/rfmpd/enable-overlays.sh

%pre
useradd --system --home-dir /var/lib/rfmpd --shell /usr/sbin/nologin rfmpd 2>/dev/null || true
usermod -a -G audio,dialout,plugdev rfmpd

%post
mkdir -p /var/lib/rfmpd /var/log/rfmpd
chown rfmpd:rfmpd /var/lib/rfmpd /var/log/rfmpd
systemctl daemon-reload
udevadm control --reload-rules
udevadm trigger
loginctl enable-linger rfmpd
# Enable SPI/I2C overlays for display hardware
if [ -f /usr/share/rfmpd/enable-overlays.sh ]; then
    . /usr/share/rfmpd/enable-overlays.sh
fi

%preun
systemctl stop rfmpd 2>/dev/null || true
systemctl disable rfmpd 2>/dev/null || true

%postun
if [ "$1" = "0" ]; then
    rm -rf /var/lib/rfmpd /var/log/rfmpd
    userdel rfmpd 2>/dev/null || true
fi
systemctl daemon-reload
