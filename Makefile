.PHONY: all build run test lint vet fmt clean

BINARY_NAME=bot

all: test build

build:
	go build -o $(BINARY_NAME) ./cmd/bot

run:
	go run ./cmd/bot --config configs/config.yaml

test:
	go test -v -race ./internal/...

lint:
	golangci-lint run ./...

vet:
	go vet ./...

fmt:
	go fmt ./...

clean:
	go clean
	rm -f $(BINARY_NAME) $(BINARY_NAME).exe

docker-build:
	docker compose build

docker-up:
	docker compose up -d

docker-down:
	docker compose down

docker-logs:
	docker compose logs -f
