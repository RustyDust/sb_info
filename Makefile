.DEFAULT_GOAL := mactel

VERSION := $(shell git tag | tail -1)

amd64: GOARCH=amd64
amd64: GOOS=linux
amd64: EXT=
amd64: build pack

arm64: GOARCH=arm64
arm64: GOOS=linux
arm64: EXT=
arm64: build pack

mactel: GOARCH=amd64
mactel: GOOS=darwin
mactel: EXT=
mactel: build pack

macarm: GOARCH=arm64
macarm: GOOS=darwin
macarm: EXT=
macarm: build pack

wintel: GOARCH=amd64
wintel: GOOS=windows
wintel: EXT=.exe
wintel: build pack

winarm: GOARCH=arm64
winarm: GOOS=windows
winarm: EXT=.exe
winarm: build pack

all: RUNS=amd64 arm64 mactel macarm wintel winarm
all:
	@for RUN in $(RUNS); do make $$RUN; done

build:
	@rm -rf bin/$(GOOS)/$(GOARCH)
	@mkdir -p bin/$(GOOS)/$(GOARCH)
	@echo -n Building sb_test in './bin/$(GOOS)/$(GOARCH)'. Please wait ...
	@GOOS=$(GOOS) GOARCH=$(GOARCH) go build -trimpath -a -o bin/$(GOOS)/$(GOARCH)/sb_test$(EXT)
	@echo " done"

pack:
	@echo Version $(VERSION) $(GOOS) $(GOARCH)
	@rm -f bin/sb_info-*-$(GOOS)-$(GOARCH).tar.gz
	@gtar czf bin/sb_info-$(VERSION)-$(GOOS)-$(GOARCH).tar.gz bin/$(GOOS)/$(GOARCH)/sb_test$(EXT)

.PHONY: build
