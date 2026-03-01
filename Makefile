.PHONY: all build frontend clean

BUILD_TAGS := production
ifeq ($(shell uname -s),Linux)
  ifneq ($(shell pkg-config --exists webkit2gtk-4.1 2>/dev/null && echo yes),)
    BUILD_TAGS += webkit2_41
  endif
endif

all: build

frontend:
	cd frontend && pnpm install && pnpm run build

build: frontend
	go build -tags "$(BUILD_TAGS)" -o pgpageshell .

clean:
	rm -rf pgpageshell frontend/dist
