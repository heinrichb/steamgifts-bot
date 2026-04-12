.PHONY: build build-windows build-all test test-cover test-cover-html lint fmt fmt-check vet tidy run check clean docker refresh-fixtures

BINARY := steamgifts-bot
PKG    := ./...

build:
	go build -trimpath -ldflags="-s -w" -o $(BINARY) ./cmd/steamgifts-bot

test:
	go test -race -count=1 $(PKG)

test-cover:
	go test -race -count=1 -coverprofile=coverage.out $(PKG)
	@go tool cover -func=coverage.out | go tool coverage-formatter

test-cover-html:
	go test -race -count=1 -coverprofile=coverage.out $(PKG)
	go tool cover -html=coverage.out

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
	rm -f $(BINARY) $(BINARY).exe coverage.out
	rm -rf dist/

build-windows:
	GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o dist/$(BINARY).exe ./cmd/steamgifts-bot

build-all: build build-windows

docker:
	docker build -t $(BINARY):dev .

refresh-fixtures:
	go run ./cmd/refresh-fixtures
