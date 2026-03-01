.PHONY: all build frontend clean

all: build

frontend:
	cd frontend && pnpm install && pnpm run build

build: frontend
	go build -o pgpageshell .

clean:
	rm -rf pgpageshell frontend/dist
