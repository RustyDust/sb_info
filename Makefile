.DEFAULT_GOAL := mactel

amd64: GOARCH=amd64
amd64: GOOS=linux
amd64: EXT=
amd64: build

arm64: GOARCH=arm64
arm64: GOOS=linux
arm64: EXT=
arm64: build

mactel: GOARCH=amd64
mactel: GOOS=darwin
mactel: EXT=
mactel: build

macarm: GOARCH=arm64
macarm: GOOS=darwin
macarm: EXT=
macarm: build

wintel: GOARCH=amd64
wintel: GOOS=windows
wintel: EXT=.exe
wintel: build

all: RUNS=amd64 arm64 mactel macarm wintel
all:
	@for RUN in $(RUNS); do make $$RUN; done

build:
	@rm -rf bin/$(GOOS)/$(GOARCH)
	@mkdir -p bin/$(GOOS)/$(GOARCH)
	@echo -n Building sb_test in './bin/$(GOOS)/$(GOARCH)'. Please wait ...
	@GOOS=$(GOOS) GOARCH=$(GOARCH) go build -trimpath -a -o bin/$(GOOS)/$(GOARCH)/sb_test$(EXT)
	@echo " done"

.PHONY: build
