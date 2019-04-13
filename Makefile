GO=go

all: income bond-yield listing

income:
	$(GO) build -o bin/$@ cmd/$@/*.go

bond-yield:
	$(GO) build -o bin/$@ cmd/$@/*.go

listing:
	$(GO) build -o bin/$@ cmd/$@/*.go

clean:
	rm -rf bin

.PHONY: clean all income bond-yield listing
