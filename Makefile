.PHONY: all clean generate

BUILDDIR := ./build
MAIN := ./cmd/pijector
PLATFORMS := linux-arm6 linux-arm7 linux-amd64 linux-386 darwin-amd64
VERSION := $(shell git describe --always --tags --dirty="-dev-$$(git rev-parse --short HEAD)")
BUILDCMD := go build -o
ifneq ($(strip $(VERSION)),)
	BUILDCMD := go build -ldflags="-X 'main.Version=$(VERSION)'" -o
endif

TARGETS := $(foreach ku,$(PLATFORMS),$(BUILDDIR)/pijector-$(ku))
SUMS := SHA1SUM.txt SHA256SUM.txt

all: $(TARGETS) $(SUMS)

clean:
	@rm -f ./client/internal/blob.go
	@rm -rf "$(BUILDDIR)"

generated:
	go generate ./...

"$(BUILDDIR)/pijector-linux-arm%":
	env GOOS=linux GOARCH=arm GOARM=$* $(BUILDCMD) $@ $(MAIN)

"$(BUILDDIR)/pijector-linux-%":
	env GOOS=linux GOARCH=$* $(BUILDCMD) $@ $(MAIN)

"$(BUILDDIR)/pijector-darwin-%":
	env GOOS=darwin GOARCH=$* $(BUILDCMD) $@ $(MAIN)

"$(BUILDDIR)/SHA%SUM.txt": $(TARGETS)
	shasum -a $* $(TARGETS) > $@

"$(BUILDDIR)/%": generated
	go build -o $@ ./cmd/$*
