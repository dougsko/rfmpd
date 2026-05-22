# Package metadata — single source of truth for all package formats
VERSION     ?= 0.5.0
NAME        := rfmpd
SUMMARY     := RFMP daemon — resilient mesh messaging over AX.25 packet radio
MAINTAINER  := Doug Prostko
LICENSE     := MIT
DEPENDS     := direwolf
CONFLICTS   := brltty

DIST        := dist

DOCKER_ARGS := --build-arg VERSION=$(VERSION) \
	--build-arg NAME=$(NAME) --build-arg "SUMMARY=$(SUMMARY)" \
	--build-arg "MAINTAINER=$(MAINTAINER)" --build-arg LICENSE=$(LICENSE) \
	--build-arg DEPENDS=$(DEPENDS) --build-arg CONFLICTS=$(CONFLICTS)

.PHONY: dist deb rpm apk tarball sha256 build clean test cover

dist: deb rpm apk tarball sha256

deb:
	@mkdir -p $(DIST)
	docker build --target deb-arm64-output $(DOCKER_ARGS) --output type=local,dest=$(DIST) .
	docker build --target deb-amd64-output $(DOCKER_ARGS) --output type=local,dest=$(DIST) .

rpm:
	@mkdir -p $(DIST)
	docker build --target rpm-arm64-output $(DOCKER_ARGS) --output type=local,dest=$(DIST) .
	docker build --target rpm-amd64-output $(DOCKER_ARGS) --output type=local,dest=$(DIST) .

apk:
	@mkdir -p $(DIST)
	docker build --target apk-arm64-output $(DOCKER_ARGS) --output type=local,dest=$(DIST) .
	docker build --target apk-amd64-output $(DOCKER_ARGS) --output type=local,dest=$(DIST) .

tarball:
	@mkdir -p $(DIST)
	docker build --target tarball-output $(DOCKER_ARGS) --output type=local,dest=$(DIST) .

sha256:
	cd $(DIST) && shasum -a 256 *.deb *.rpm *.apk *.tar.gz > SHA256SUMS

build:
	go build -ldflags="-s -w -X main.version=$(VERSION)" -o $(NAME) .

test:
	go test ./...

cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Coverage report: coverage.html"

clean:
	rm -rf $(DIST) $(NAME) coverage.out coverage.html
