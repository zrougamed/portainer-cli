BINARY=portainer-tui
MAIN=./cmd

.PHONY: build run install tidy clean

build:
	go build -o $(BINARY) $(MAIN)

run:
	go run $(MAIN)/...

install:
	go install $(MAIN)

tidy:
	go mod tidy

clean:
	rm -f $(BINARY)

# Quick login helper
login:
	go run $(MAIN)/... login

# Open Portainer in browser
open:
	go run $(MAIN)/... open
