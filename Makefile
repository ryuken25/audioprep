# audioprep build helpers.
#
# On Windows, run these from Git Bash or MSYS2 (they need a POSIX shell). You
# need Go 1.22+ and a C compiler on PATH (mingw-w64 gcc); see README.md.
#
#   make test        run unit tests (-short skips the ffmpeg smoke tests)
#   make smoke       run the ffmpeg integration tests (needs ffmpeg on PATH)
#   make build       build dist/audioprep-windows-amd64.exe (no console window)
#   make icon        regenerate assets/icon.png from tools/genicon
#   make winres      regenerate the Windows icon/manifest .syso files
#   make web         build the browser version into web/dist

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
# Plain numeric version for the Windows resource block (git describe output is not valid there).
WINRES_VERSION ?= 0.4.0
LDFLAGS  = -H windowsgui -s -w -X main.version=$(VERSION)
BIN      = dist/audioprep-windows-amd64.exe

.PHONY: all test smoke vet build icon winres web clean

all: vet test build

test:
	go test -short ./...

smoke:
	go test ./internal/pipeline -run Integration -v

vet:
	go vet ./...

build:
	mkdir -p dist
	CGO_ENABLED=1 GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN) ./cmd/audioprep
	@ls -la $(BIN)

icon:
	go run tools/genicon/main.go assets/icon.png

winres:
	go run github.com/tc-hib/go-winres@latest simply \
		--icon assets/icon.png --manifest gui \
		--product-name audioprep \
		--file-description "Audio-first video encoder for X and TikTok" \
		--product-version $(WINRES_VERSION) --file-version $(WINRES_VERSION).0 \
		--copyright "MIT License, ryuken25" \
		--original-filename audioprep.exe \
		--out cmd/audioprep/rsrc

web:
	cd web && npm ci && npm run build

clean:
	rm -rf dist
