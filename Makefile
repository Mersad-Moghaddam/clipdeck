SHELL := /bin/bash
VERSION ?= 0.4.0
ARCH ?= amd64
LDFLAGS := -s -w -X main.version=$(VERSION)
PREFIX ?= $(shell echo $$HOME)/.local
DIST := dist
NFPM ?= $(shell command -v nfpm 2>/dev/null || echo /tmp/opencode/bin/nfpm)

.PHONY: all build test race bench lint integ release clean packages deb rpm apk tarball install uninstall

all: build test

build:
	CGO_ENABLED=0 GOOS=linux GOARCH=$(ARCH) go build -trimpath -ldflags '$(LDFLAGS)' -o bin/clipdeckd ./cmd/clipdeckd
	CGO_ENABLED=0 GOOS=linux GOARCH=$(ARCH) go build -trimpath -ldflags '$(LDFLAGS)' -o bin/clipdeck ./cmd/clipdeck

test:
	go test -v ./...

race:
	go test -race -v ./...

bench:
	mkdir -p reports
	go test -bench=. -benchmem ./internal/store > reports/bench-search.txt 2>&1

lint:
	go vet ./...

integ:
	go test -v -tags=integration ./...

# Named packages via nfpm plus a portable tarball. The binaries are CGO-free,
# so every artifact works on any distro of the matching architecture.
packages: deb rpm apk tarball

deb rpm apk: build
	@mkdir -p $(DIST)
	VERSION=$(VERSION) ARCH=$(ARCH) $(NFPM) package --config packaging/nfpm.yaml --packager $@ --target $(DIST)/

tarball: build
	@mkdir -p $(DIST)
	tar --owner=0 --group=0 -C bin -czf $(DIST)/clipdeck_$(VERSION)_linux_$(ARCH).tar.gz clipdeck clipdeckd
	cd $(DIST) && sha256sum clipdeck_$(VERSION)_linux_$(ARCH).tar.gz > clipdeck_$(VERSION)_linux_$(ARCH).tar.gz.sha256

release: packages
	@echo "Release artifacts for $(VERSION) in $(DIST)/"

install: build
	install -d $(PREFIX)/bin
	install -m 755 bin/clipdeckd $(PREFIX)/bin/clipdeckd
	install -m 755 bin/clipdeck $(PREFIX)/bin/clipdeck
	install -d $(HOME)/.config/systemd/user
	sed 's|/usr/bin/clipdeckd|$(PREFIX)/bin/clipdeckd|' packaging/clipdeckd.service > $(HOME)/.config/systemd/user/clipdeckd.service
	systemctl --user daemon-reload

uninstall:
	-systemctl --user disable --now clipdeckd.service
	rm -f $(HOME)/.config/systemd/user/clipdeckd.service $(PREFIX)/bin/clipdeck $(PREFIX)/bin/clipdeckd
	-systemctl --user daemon-reload

clean:
	rm -rf bin/ coverage.out $(DIST)
