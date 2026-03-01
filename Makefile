.PHONY: all build generate clean

all: build

generate:
	go generate ./...

build: generate
	go build -o pgpageshell .

clean:
	rm -rf pgpageshell web/dist
