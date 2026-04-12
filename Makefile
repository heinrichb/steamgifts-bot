.PHONY: build test lint fmt fmt-check vet tidy run check clean docker refresh-fixtures

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
	npx --yes prettier --write '**/*.{yml,md,json}' --ignore-path .prettierignore

fmt-check:
	@echo "=== gofmt ==="
	@test -z "$$(gofmt -l .)" || (gofmt -l . && exit 1)
	@echo "=== prettier ==="
	npx --yes prettier --check '**/*.{yml,md,json}' --ignore-path .prettierignore

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

refresh-fixtures:
	go run ./cmd/refresh-fixtures
