PREFIX ?= /usr/local
VERSION := $(shell cat VERSION)

.PHONY: all build install clean release

all: build

build:
	go build -ldflags="-X main.version=$(VERSION)" -o bumpwf .

install: build
	sudo install -m 755 bumpwf $(PREFIX)/bin/bumpwf

clean:
	rm -f bumpwf

release:
	$(MAKE) build
	git tag $(VERSION)
	git push origin $(VERSION)
