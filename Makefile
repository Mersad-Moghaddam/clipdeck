SHELL := /bin/bash
VERSION ?= 0.1.0
LDFLAGS := -s -w -X main.Version=$(VERSION)
PREFIX ?= $(shell echo $$HOME)/.local

.PHONY: all build test race bench lint integ release clean

all: build test

build:
	CGO_ENABLED=0 go build -trimpath -ldflags '$(LDFLAGS)' -o bin/clipdeckd ./cmd/clipdeckd
	CGO_ENABLED=0 go build -trimpath -ldflags '$(LDFLAGS)' -o bin/clipdeck ./cmd/clipdeck

test:
	go test -v ./...

race:
	go test -race -v ./...

bench:
	mkdir -p reports
	go test -bench=. -benchmem ./internal/store > reports/bench-search.txt 2>&1 || true

lint:
	go vet ./...

integ:
	go test -v -tags=integration ./...

release: build
	@echo "Built release binaries for $(VERSION) in bin/"

install: build
	install -d $(PREFIX)/bin
	install -m 755 bin/clipdeckd $(PREFIX)/bin/clipdeckd
	install -m 755 bin/clipdeck $(PREFIX)/bin/clipdeck
	install -d $(HOME)/.config/systemd/user
	sed 's|/usr/bin/clipdeckd|$(PREFIX)/bin/clipdeckd|' packaging/clipdeckd.service > $(HOME)/.config/systemd/user/clipdeckd.service
	systemctl --user daemon-reload

clean:
	rm -rf bin/ coverage.out
