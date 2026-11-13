RUNNERS := $(wildcard runners/*)

.PHONY: all $(RUNNERS)
all: $(RUNNERS)

$(RUNNERS): %:
	go build -o $@/main ./$@
