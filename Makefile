.PHONY: default test build

OS := $(shell uname)
VERSION ?= 1.0.0
APPNAME := sync
MAIN := main.go

# target #

default: unit-test integration-test build

build:
	mkdir -p bin
	@echo "Setup sync"
ifeq ($(OS), Linux)
	@echo "Build sync..."
	GOOS=linux  go build -ldflags "-s -w -X main.Version=$(VERSION)" -o ./bin/$(APPNAME) $(MAIN)
else
	@echo "Build $(OS)"
	go build -ldflags "-s -w -X main.Version=$(VERSION)" -o ./bin/$(APPNAME) $(MAIN)
endif
ifeq ($(OS), Darwin)
	@echo "Build sync..."
	GOOS=darwin go build -ldflags "-X main.Version=$(VERSION)" -o ./bin/$(APPNAME) $(MAIN)
endif
ifeq ($(OS),Windows_NT)
	GOOS=windows GOARCH=amd64 go build -o ./bin/$(APPNAME).exe main.go
endif
	@echo "Succesfully Build for ${OS} version:= ${VERSION}"

# Test Packages

unit-test:
	@go test -count=1 -v --cover ./... -tags="unit"

integration-test:
 	@go test -count=1 -v --cover -tags="integration" -p 1 ./... --env-path=.env
