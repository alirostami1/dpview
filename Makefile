.PHONY: build test check run dev

build:
	 go build -o build/dpview ./app/cmd

test:
	 go test ./...

check:
	 go build ./...

run:
	 go run ./app/cmd --root .

dev:
	 go tool air -c .air.toml
