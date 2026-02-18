BINARY_NAME=cmdvault
BUILD_DIR=./cmd/cmdvault
INSTALL_DIR=$(HOME)/.local/bin
VERSION=$(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS=-ldflags "-X main.Version=$(VERSION)"

.PHONY: build install uninstall clean

build:
	go build $(LDFLAGS) -o $(BINARY_NAME) $(BUILD_DIR)

install: build
	mkdir -p $(INSTALL_DIR)
	cp $(BINARY_NAME) $(INSTALL_DIR)/$(BINARY_NAME)
	@echo "Installed to $(INSTALL_DIR)/$(BINARY_NAME)"

uninstall:
	rm -f $(INSTALL_DIR)/$(BINARY_NAME)
	@echo "Removed $(INSTALL_DIR)/$(BINARY_NAME)"

clean:
	rm -f $(BINARY_NAME)
