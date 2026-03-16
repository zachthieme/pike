.PHONY: build test lint bench fuzz cover golden-update install

build:
	go build -o pike ./cmd/pike

test:
	go test -race -count=1 ./...

lint:
	golangci-lint run

bench:
	go test -bench=. -benchmem ./...

fuzz:
	go test -fuzz=. -fuzztime=30s ./internal/parser
	go test -fuzz=. -fuzztime=30s ./internal/query

cover:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out

golden-update:
	go test -run=TestGolden -update .

install:
	go install ./cmd/pike
