.PHONY: check build

check:
	test -z "$$(gofmt -l .)"
	go test -race ./...
	go vet ./...

build:
	go build -trimpath -o bin/mail-server ./cmd/mail-server
