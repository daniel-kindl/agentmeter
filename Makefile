ifeq ($(OS),Windows_NT)
SHELL := C:/PROGRA~1/Git/bin/sh.exe
else
SHELL := sh
endif

VERSION ?= dev
LDFLAGS := -X main.version=$(VERSION)

.PHONY: build check cross fmt hooks lint test vet

build:
	mkdir bin 2>/dev/null || true
	go build -ldflags "$(LDFLAGS)" -o bin/agentmeter ./cmd/agentmeter

check: fmt vet lint test

cross:
	mkdir dist 2>/dev/null || true
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/agentmeter-windows-amd64.exe ./cmd/agentmeter
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o dist/agentmeter-linux-amd64 ./cmd/agentmeter

fmt:
	unformatted=$$(find . -name '*.go' -not -path './.git/*' -exec gofmt -l {} +); \
		test -z "$$unformatted" || { printf 'unformatted files:\n%s\n' "$$unformatted"; exit 1; }

hooks:
	git config core.hooksPath .githooks
	git config commit.template .gitmessage

lint:
	packages=$$(go list ./... 2>/dev/null || true); \
		test -z "$$packages" || golangci-lint run ./...

test:
	packages=$$(go list ./... 2>/dev/null || true); \
		test -z "$$packages" || go test -race -coverprofile=coverage.out ./...

vet:
	packages=$$(go list ./... 2>/dev/null || true); \
		test -z "$$packages" || go vet ./...
