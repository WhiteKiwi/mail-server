.PHONY: check build

check:
	test -z "$$(gofmt -l .)"
	go test -race -p=1 -parallel=1 ./...
	go vet ./...

build:
	go build -trimpath -o bin/mail-server ./cmd/mail-server
