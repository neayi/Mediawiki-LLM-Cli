VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null | sed 's/^v//' || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)
PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64

.PHONY: build test dist clean

build:
	go build -ldflags="$(LDFLAGS)" -o mediawiki ./cmd/mediawiki

test:
	go vet ./...
	go test ./...

# Cross-compiles static binaries for every platform into dist/. File names
# carry no version, so that the install scripts can always download
# releases/latest/download/mediawiki-<os>-<arch>.
dist:
	@rm -rf dist
	@for p in $(PLATFORMS); do \
		os=$${p%/*}; arch=$${p#*/}; ext=; [ $$os = windows ] && ext=.exe; \
		out=dist/mediawiki-$$os-$$arch$$ext; \
		echo $$out; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build -trimpath -ldflags="$(LDFLAGS)" -o $$out ./cmd/mediawiki || exit 1; \
	done
	cd dist && sha256sum mediawiki-* > checksums.txt

clean:
	rm -rf dist mediawiki mediawiki.exe
