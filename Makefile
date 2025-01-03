subdir = $(shell ls -l | grep ^d | awk '{print $$9}')

.PHONY: build all test clean

all: build build-unix build-ubuntu

build:
	go build -ldflags "-s -w" -o lotus-car ./*.go

build-unix:
	GOOS=darwin GOARCH=amd64 go build -ldflags "-s -w" -o lotus-car-darwin-amd64 ./*.go

build-ubuntu:
	GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o lotus-car-linux-amd64 ./*.go

install:
	install -C ./lotus-car /usr/local/bin/lotus-car

test:
	bundle2.7 exec rspec -f d

clean:
	rm -rfv ./lotus-car ./lotus-car-linux-amd64 ./lotus-car-darwin-amd64
