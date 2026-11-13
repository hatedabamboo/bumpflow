PREFIX ?= /usr/local
VERSION := $(shell cat VERSION)

.PHONY: all build install clean bump-patch bump-minor bump-major

all: build

build:
	go build -ldflags="-X main.version=$(VERSION)" -o bumpflow .

install: build
	sudo install -m 755 bumpflow $(PREFIX)/bin/bumpflow

clean:
	rm -f bumpflow

bump-patch:
	    @$(call bump,patch)

bump-minor:
	    @$(call bump,minor)

bump-major:
	    @$(call bump,major)

define bump
    cur="$$(cat VERSION | tr -d '[:space:]')"; \
    v="$${cur#v}"; maj="$${v%%.*}"; rest="$${v#*.}"; min="$${rest%%.*}"; pat="$${rest#*.}"; \
    case "$(1)" in \
        major) maj=$$((maj+1)); min=0; pat=0 ;; \
        minor) min=$$((min+1)); pat=0 ;; \
        patch) pat=$$((pat+1)) ;; \
    esac; \
    new="v$$maj.$$min.$$pat"; \
    echo "$$new" > VERSION; \
    echo "$$cur -> $$new"
endef
