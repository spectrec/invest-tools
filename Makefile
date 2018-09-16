GO=go

all: bond-listing bond-yield

bond-yield:
	$(GO) build -o $@.bin $@/*.go

bond-listing:
	$(GO) build -o $@.bin $@/*.go

clean:
	rm -f *.bin

.PHONY: clean all bond-listing bond-yield
