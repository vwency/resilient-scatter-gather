BINARY_NAME=api-gateway
MAIN_FILE=cmd/main.go

.PHONY: all deps build run test clean

all: build

deps:
	go mod tidy

build: deps
	go build -o $(BINARY_NAME) $(MAIN_FILE)

run: build
	./$(BINARY_NAME)

test:
	go test ./tests/handlers -v

clean:
	rm -f $(BINARY_NAME)
