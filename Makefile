.PHONY: build run test lint docker-build release clean

BINARY_NAME=cybarbot
GO_FILES=$(shell find . -name '*.go')

build:
	go build -o $(BINARY_NAME) ./cmd/cybarbot

run: build
	./$(BINARY_NAME)

test:
	go test -v -cover ./...

lint:
	golangci-lint run

docker-build:
	docker build -t cybarbot:latest .

release:
	goreleaser release --snapshot --clean

clean:
	rm -f $(BINARY_NAME)
