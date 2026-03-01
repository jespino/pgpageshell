.PHONY: all build frontend clean

BUILD_TAGS := production
CGO_LDFLAGS_EXTRA :=

ifeq ($(shell uname -s),Linux)
  ifneq ($(shell pkg-config --exists webkit2gtk-4.1 2>/dev/null && echo yes),)
    BUILD_TAGS += webkit2_41
  endif
endif

ifeq ($(shell uname -s),Darwin)
  CGO_LDFLAGS_EXTRA := -framework UniformTypeIdentifiers
endif

all: build

frontend:
	cd frontend && pnpm install && pnpm run build

build: frontend
	CGO_LDFLAGS="$(CGO_LDFLAGS_EXTRA)" go build -tags "$(BUILD_TAGS)" -o pgpageshell .

clean:
	rm -rf pgpageshell frontend/dist
