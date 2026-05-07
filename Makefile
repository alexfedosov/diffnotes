.PHONY: build build-linux test vet clean

build:
	go build -o bin/git-review-tui ./cmd/git-review-tui

build-linux:
	GOOS=linux GOARCH=amd64 go build -o bin/git-review-tui-linux-amd64 ./cmd/git-review-tui

test:
	go test ./...

vet:
	go vet ./...

clean:
	rm -rf bin
