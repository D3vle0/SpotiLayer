.PHONY: all build run clean

BINARY_NAME=spotify-obs

all: build

build:
	go build -ldflags="-s -w" -o $(BINARY_NAME) .

run:
	go run .

clean:
	rm -f $(BINARY_NAME) $(BINARY_NAME).exe config.json
