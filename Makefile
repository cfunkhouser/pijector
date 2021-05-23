.PHONY: all clean generated

BUILDDIR := ./build
MAIN := ./cmd/pijector
PLATFORMS := linux-arm6 linux-arm7 linux-amd64 linux-386 darwin-amd64
VERSION := $(shell git describe --always --tags --dirty="-dev-$$(git rev-parse --short HEAD)")
BUILDCMD := go build -o
ifneq ($(strip $(VERSION)),)
	BUILDCMD := go build -ldflags="-X 'pijector.Version=$(VERSION)'" -o
endif

BINARIES := $(foreach ku,$(PLATFORMS),$(BUILDDIR)/pijector-$(ku))
SUMS := $(foreach ku,SHA1SUM.txt SHA256SUM.txt,$(BUILDDIR)/$(ku))

all: generated $(BINARIES) $(SUMS)

clean:
	@rm -f ./admin/internal/staticfiles.go
	@rm -rf "$(BUILDDIR)"

generated:
	go generate ./...

$(BUILDDIR)/pijector-linux-arm%:
	env GOOS=linux GOARCH=arm GOARM=$* $(BUILDCMD) $@ $(MAIN)

$(BUILDDIR)/pijector-linux-%:
	env GOOS=linux GOARCH=$* $(BUILDCMD) $@ $(MAIN)

$(BUILDDIR)/pijector-darwin-%:
	env GOOS=darwin GOARCH=$* $(BUILDCMD) $@ $(MAIN)

$(BUILDDIR)/SHA%SUM.txt: $(BINARIES)
	shasum -a $* $(BINARIES) > $@

$(BUILDDIR)/%: generated
	go build -o $@ ./cmd/$*
