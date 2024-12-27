#!/bin/sh
.PHONY: build
default: build


install:
	go install smart-testify/cmd/smart-testify

test:
	go test -v -short ./... || exit
