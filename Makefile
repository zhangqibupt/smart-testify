#!/bin/sh
.PHONY: build
default: build


install:
	@echo "Installing smart-testify..."
	go install smart-testify/cmd/smart-testify
	@which goimports > /dev/null || (echo "Installing goimports..." && go install golang.org/x/tools/cmd/goimports@latest)
	@echo "Installation complete!"

test:
	go test -v -short ./... || exit
