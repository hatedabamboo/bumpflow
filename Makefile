PREFIX ?= /usr/local
VERSION := $(shell cat VERSION)

.PHONY: all build install clean release

all: build

build:
	go build -ldflags="-X main.version=$(VERSION)" -o bumpflow .

install: build
	sudo install -m 755 bumpflow $(PREFIX)/bin/bumpflow

clean:
	rm -f bumpflow

release:
	$(MAKE) build
	git tag $(VERSION)
	git push origin $(VERSION)
