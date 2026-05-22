# Multi-stage build for rfmpd packages
# Usage: invoked by Makefile targets (make dist)

ARG VERSION=0.5.0
ARG NAME=rfmpd
ARG SUMMARY="RFMP daemon"
ARG MAINTAINER="Doug Prostko"
ARG LICENSE=MIT
ARG DEPENDS=direwolf
ARG CONFLICTS=brltty

# ============================================================
# Stage: Build Go binaries for both architectures
# ============================================================
FROM golang:1.24-bookworm AS build-arm64
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION
RUN GOOS=linux GOARCH=arm64 CGO_ENABLED=0 \
    go build -ldflags="-s -w -X main.version=${VERSION}" -o /out/rfmpd .

FROM golang:1.24-bookworm AS build-amd64
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
ARG VERSION
RUN GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
    go build -ldflags="-s -w -X main.version=${VERSION}" -o /out/rfmpd .

# ============================================================
# Stage: Assemble .deb packages
# ============================================================
FROM debian:bookworm-slim AS deb-arm64
ARG VERSION
ARG NAME
ARG SUMMARY
ARG MAINTAINER
ARG LICENSE
ARG DEPENDS
ARG CONFLICTS
WORKDIR /work
COPY --from=build-arm64 /out/rfmpd ./rfmpd
COPY config.yaml.example ./config.yaml
COPY radio/direwolf-*.conf ./
COPY radio/99-rfmp-radio.rules ./
COPY packaging/systemd/ ./systemd/
COPY packaging/deb/ ./deb/
COPY packaging/scripts/enable-overlays.sh ./enable-overlays.sh
RUN mkdir -p /out pkg/DEBIAN pkg/usr/bin pkg/etc/rfmpd/direwolf \
      pkg/lib/systemd/system pkg/usr/lib/udev/rules.d pkg/usr/share/rfmpd && \
    cp rfmpd pkg/usr/bin/rfmpd && chmod 755 pkg/usr/bin/rfmpd && \
    cp config.yaml pkg/etc/rfmpd/config.yaml && \
    cp direwolf-*.conf pkg/etc/rfmpd/direwolf/ && \
    cp systemd/rfmpd.service pkg/lib/systemd/system/ && \
    cp systemd/direwolf@.service pkg/lib/systemd/system/ && \
    cp 99-rfmp-radio.rules pkg/usr/lib/udev/rules.d/ && \
    cp enable-overlays.sh pkg/usr/share/rfmpd/enable-overlays.sh && \
    cp deb/conffiles pkg/DEBIAN/conffiles && \
    cp deb/postinst pkg/DEBIAN/postinst && chmod 755 pkg/DEBIAN/postinst && \
    cp deb/prerm pkg/DEBIAN/prerm && chmod 755 pkg/DEBIAN/prerm && \
    cp deb/postrm pkg/DEBIAN/postrm && chmod 755 pkg/DEBIAN/postrm && \
    sed -e "s/\${NAME}/${NAME}/" -e "s/\${VERSION}/${VERSION}/" \
        -e "s/\${ARCH}/arm64/" -e "s/\${MAINTAINER}/${MAINTAINER}/" \
        -e "s/\${SUMMARY}/${SUMMARY}/" -e "s/\${LICENSE}/${LICENSE}/" \
        -e "s/\${DEPENDS}/${DEPENDS}/" -e "s/\${CONFLICTS}/${CONFLICTS}/" \
        deb/control.tmpl > pkg/DEBIAN/control && \
    dpkg-deb --build pkg /out/${NAME}_${VERSION}_arm64.deb
FROM scratch AS deb-arm64-output
COPY --from=deb-arm64 /out/ /

FROM debian:bookworm-slim AS deb-amd64
ARG VERSION
ARG NAME
ARG SUMMARY
ARG MAINTAINER
ARG LICENSE
ARG DEPENDS
ARG CONFLICTS
WORKDIR /work
COPY --from=build-amd64 /out/rfmpd ./rfmpd
COPY config.yaml.example ./config.yaml
COPY radio/direwolf-*.conf ./
COPY radio/99-rfmp-radio.rules ./
COPY packaging/systemd/ ./systemd/
COPY packaging/deb/ ./deb/
COPY packaging/scripts/enable-overlays.sh ./enable-overlays.sh
RUN mkdir -p /out pkg/DEBIAN pkg/usr/bin pkg/etc/rfmpd/direwolf \
      pkg/lib/systemd/system pkg/usr/lib/udev/rules.d pkg/usr/share/rfmpd && \
    cp rfmpd pkg/usr/bin/rfmpd && chmod 755 pkg/usr/bin/rfmpd && \
    cp config.yaml pkg/etc/rfmpd/config.yaml && \
    cp direwolf-*.conf pkg/etc/rfmpd/direwolf/ && \
    cp systemd/rfmpd.service pkg/lib/systemd/system/ && \
    cp systemd/direwolf@.service pkg/lib/systemd/system/ && \
    cp 99-rfmp-radio.rules pkg/usr/lib/udev/rules.d/ && \
    cp enable-overlays.sh pkg/usr/share/rfmpd/enable-overlays.sh && \
    cp deb/conffiles pkg/DEBIAN/conffiles && \
    cp deb/postinst pkg/DEBIAN/postinst && chmod 755 pkg/DEBIAN/postinst && \
    cp deb/prerm pkg/DEBIAN/prerm && chmod 755 pkg/DEBIAN/prerm && \
    cp deb/postrm pkg/DEBIAN/postrm && chmod 755 pkg/DEBIAN/postrm && \
    sed -e "s/\${NAME}/${NAME}/" -e "s/\${VERSION}/${VERSION}/" \
        -e "s/\${ARCH}/amd64/" -e "s/\${MAINTAINER}/${MAINTAINER}/" \
        -e "s/\${SUMMARY}/${SUMMARY}/" -e "s/\${LICENSE}/${LICENSE}/" \
        -e "s/\${DEPENDS}/${DEPENDS}/" -e "s/\${CONFLICTS}/${CONFLICTS}/" \
        deb/control.tmpl > pkg/DEBIAN/control && \
    dpkg-deb --build pkg /out/${NAME}_${VERSION}_amd64.deb
FROM scratch AS deb-amd64-output
COPY --from=deb-amd64 /out/ /

# ============================================================
# Stage: Assemble .rpm packages
# ============================================================
FROM debian:bookworm-slim AS rpm-arm64
RUN apt-get update && apt-get install -y rpm && rm -rf /var/lib/apt/lists/*
ARG VERSION
ARG NAME
ARG SUMMARY
ARG MAINTAINER
ARG LICENSE
ARG DEPENDS
ARG CONFLICTS
WORKDIR /work
COPY --from=build-arm64 /out/rfmpd ./rfmpd
COPY config.yaml.example ./config.yaml
COPY radio/direwolf-*.conf ./
COPY radio/99-rfmp-radio.rules ./
COPY packaging/systemd/ ./systemd/
COPY packaging/rpm/rfmpd.spec ./rfmpd.spec
COPY packaging/scripts/enable-overlays.sh ./enable-overlays.sh
RUN mkdir -p /out /root/rpmbuild/SOURCES /root/rpmbuild/SPECS /root/rpmbuild/BUILD /root/rpmbuild/RPMS /root/rpmbuild/SRPMS && \
    cp rfmpd config.yaml direwolf-*.conf 99-rfmp-radio.rules enable-overlays.sh \
       systemd/rfmpd.service systemd/direwolf@.service /root/rpmbuild/SOURCES/ && \
    sed -e "s/%{pkg_name}/${NAME}/" -e "s/%{pkg_version}/${VERSION}/" \
        -e "s/%{pkg_summary}/${SUMMARY}/" -e "s/%{pkg_license}/${LICENSE}/" \
        -e "s/%{pkg_depends}/${DEPENDS}/" -e "s/%{pkg_conflicts}/${CONFLICTS}/" \
        rfmpd.spec > /root/rpmbuild/SPECS/rfmpd.spec && \
    rpmbuild -bb --target aarch64 /root/rpmbuild/SPECS/rfmpd.spec && \
    cp /root/rpmbuild/RPMS/aarch64/*.rpm /out/
FROM scratch AS rpm-arm64-output
COPY --from=rpm-arm64 /out/ /

FROM debian:bookworm-slim AS rpm-amd64
RUN apt-get update && apt-get install -y rpm && rm -rf /var/lib/apt/lists/*
ARG VERSION
ARG NAME
ARG SUMMARY
ARG MAINTAINER
ARG LICENSE
ARG DEPENDS
ARG CONFLICTS
WORKDIR /work
COPY --from=build-amd64 /out/rfmpd ./rfmpd
COPY config.yaml.example ./config.yaml
COPY radio/direwolf-*.conf ./
COPY radio/99-rfmp-radio.rules ./
COPY packaging/systemd/ ./systemd/
COPY packaging/rpm/rfmpd.spec ./rfmpd.spec
COPY packaging/scripts/enable-overlays.sh ./enable-overlays.sh
RUN mkdir -p /out /root/rpmbuild/SOURCES /root/rpmbuild/SPECS /root/rpmbuild/BUILD /root/rpmbuild/RPMS /root/rpmbuild/SRPMS && \
    cp rfmpd config.yaml direwolf-*.conf 99-rfmp-radio.rules enable-overlays.sh \
       systemd/rfmpd.service systemd/direwolf@.service /root/rpmbuild/SOURCES/ && \
    sed -e "s/%{pkg_name}/${NAME}/" -e "s/%{pkg_version}/${VERSION}/" \
        -e "s/%{pkg_summary}/${SUMMARY}/" -e "s/%{pkg_license}/${LICENSE}/" \
        -e "s/%{pkg_depends}/${DEPENDS}/" -e "s/%{pkg_conflicts}/${CONFLICTS}/" \
        rfmpd.spec > /root/rpmbuild/SPECS/rfmpd.spec && \
    rpmbuild -bb --target x86_64 /root/rpmbuild/SPECS/rfmpd.spec && \
    cp /root/rpmbuild/RPMS/x86_64/*.rpm /out/
FROM scratch AS rpm-amd64-output
COPY --from=rpm-amd64 /out/ /

# ============================================================
# Stage: Assemble .apk packages
# ============================================================
FROM alpine:3.20 AS apk-arm64
RUN apk add --no-cache alpine-sdk sudo
ARG VERSION
ARG NAME
ARG SUMMARY
ARG MAINTAINER
ARG LICENSE
ARG DEPENDS
WORKDIR /work
COPY --from=build-arm64 /out/rfmpd ./rfmpd
COPY config.yaml.example ./config.yaml
COPY radio/direwolf-*.conf ./
COPY packaging/openrc/rfmpd ./openrc-rfmpd
COPY packaging/openrc/direwolf ./openrc-direwolf
COPY packaging/mdev/rfmp-radio.conf ./rfmp-radio.conf
COPY packaging/scripts/enable-overlays.sh ./enable-overlays.sh
COPY packaging/apk/ ./apk/
RUN mkdir -p /out && \
    sed -e "s/\${NAME}/${NAME}/" -e "s/\${VERSION}/${VERSION}/" \
        -e "s/\${SUMMARY}/${SUMMARY}/" -e "s/\${LICENSE}/${LICENSE}/" \
        -e "s/\${DEPENDS}/${DEPENDS}/" -e 's/\${ARCH}/aarch64/' \
        apk/APKBUILD.tmpl > APKBUILD && \
    echo "srcdir=/work" >> APKBUILD && \
    echo "pkgdir=/work/pkg" >> APKBUILD && \
    mkdir -p pkg && \
    . ./APKBUILD && package && \
    cp apk/post-install pkg/.post-install && \
    cp apk/pre-deinstall pkg/.pre-deinstall && \
    cp apk/post-deinstall pkg/.post-deinstall && \
    cd pkg && tar czf /out/${NAME}-${VERSION}-1-aarch64.apk .
FROM scratch AS apk-arm64-output
COPY --from=apk-arm64 /out/ /

FROM alpine:3.20 AS apk-amd64
RUN apk add --no-cache alpine-sdk sudo
ARG VERSION
ARG NAME
ARG SUMMARY
ARG MAINTAINER
ARG LICENSE
ARG DEPENDS
WORKDIR /work
COPY --from=build-amd64 /out/rfmpd ./rfmpd
COPY config.yaml.example ./config.yaml
COPY radio/direwolf-*.conf ./
COPY packaging/openrc/rfmpd ./openrc-rfmpd
COPY packaging/openrc/direwolf ./openrc-direwolf
COPY packaging/mdev/rfmp-radio.conf ./rfmp-radio.conf
COPY packaging/scripts/enable-overlays.sh ./enable-overlays.sh
COPY packaging/apk/ ./apk/
RUN mkdir -p /out && \
    sed -e "s/\${NAME}/${NAME}/" -e "s/\${VERSION}/${VERSION}/" \
        -e "s/\${SUMMARY}/${SUMMARY}/" -e "s/\${LICENSE}/${LICENSE}/" \
        -e "s/\${DEPENDS}/${DEPENDS}/" -e 's/\${ARCH}/x86_64/' \
        apk/APKBUILD.tmpl > APKBUILD && \
    echo "srcdir=/work" >> APKBUILD && \
    echo "pkgdir=/work/pkg" >> APKBUILD && \
    mkdir -p pkg && \
    . ./APKBUILD && package && \
    cp apk/post-install pkg/.post-install && \
    cp apk/pre-deinstall pkg/.pre-deinstall && \
    cp apk/post-deinstall pkg/.post-deinstall && \
    cd pkg && tar czf /out/${NAME}-${VERSION}-1-x86_64.apk .
FROM scratch AS apk-amd64-output
COPY --from=apk-amd64 /out/ /

# ============================================================
# Stage: Assemble tarballs
# ============================================================
FROM debian:bookworm-slim AS tarball
ARG VERSION
ARG NAME
WORKDIR /work
COPY --from=build-arm64 /out/rfmpd ./arm64/rfmpd
COPY --from=build-amd64 /out/rfmpd ./amd64/rfmpd
COPY config.yaml.example ./config.yaml
COPY radio/direwolf-*.conf ./
COPY radio/99-rfmp-radio.rules ./
COPY packaging/systemd/ ./systemd/
COPY packaging/INSTALL.md ./INSTALL.md
RUN mkdir -p /out && \
    for arch in arm64 amd64; do \
      dir="${NAME}-${VERSION}-linux-${arch}"; \
      mkdir -p "${dir}/direwolf" "${dir}/systemd" "${dir}/udev"; \
      cp "${arch}/rfmpd" "${dir}/rfmpd"; \
      cp config.yaml "${dir}/config.yaml"; \
      cp direwolf-*.conf "${dir}/direwolf/"; \
      cp systemd/rfmpd.service systemd/direwolf@.service "${dir}/systemd/"; \
      cp 99-rfmp-radio.rules "${dir}/udev/"; \
      cp INSTALL.md "${dir}/INSTALL.md"; \
      tar czf "/out/${dir}.tar.gz" "${dir}"; \
    done
FROM scratch AS tarball-output
COPY --from=tarball /out/ /
