RUNNERS := $(wildcard runners/*)

.PHONY: all $(RUNNERS)
all: $(RUNNERS)

$(RUNNERS): %:
	go build -o $@/main ./$@

.PHONY: spread
spread:
	git clone https://github.com/nulltrek/spread-plus .dev/spread-plus
	cd .dev/spread-plus
	make build
  cd ..
  cp .dev/spread-plus/spread-plus bin/spread
