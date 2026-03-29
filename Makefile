.PHONY: build test check run dev

build:
	 go build -o build/dpview ./cmd/dpview

test:
	 go test ./...

check:
	 go build ./...

run:
	 go run ./cmd/dpview --root .

dev:
	 go tool air -c .air.toml
