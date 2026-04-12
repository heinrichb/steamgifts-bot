.PHONY: build test lint fmt vet tidy run check clean docker

BINARY := steamgifts-bot
PKG    := ./...

build:
	go build -trimpath -ldflags="-s -w" -o $(BINARY) ./cmd/steamgifts-bot

test:
	go test -race -count=1 $(PKG)

lint:
	golangci-lint run

fmt:
	gofmt -s -w .
	go run golang.org/x/tools/cmd/goimports@latest -w .

vet:
	go vet $(PKG)

tidy:
	go mod tidy

run: build
	./$(BINARY) run --once --dry-run --log-level debug

check: build
	./$(BINARY) check

clean:
	rm -f $(BINARY) $(BINARY).exe
	rm -rf dist/

docker:
	docker build -t $(BINARY):dev .
