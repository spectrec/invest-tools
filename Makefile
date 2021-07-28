GO=go

all: income fund-yield listing financials

income:
	$(GO) build -o bin/$@ cmd/$@/*.go

fund-yield:
	$(GO) build -o bin/$@ cmd/$@/*.go

listing:
	$(GO) build -o bin/$@ cmd/$@/*.go

financials:
	$(GO) build -o bin/$@ cmd/$@/*.go

clean:
	rm -rf bin

.PHONY: clean all income bond-yield fund-yield listing financials
