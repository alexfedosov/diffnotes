.PHONY: build build-linux test vet clean

build:
	go build -o bin/diffnotes ./cmd/diffnotes

build-linux:
	GOOS=linux GOARCH=amd64 go build -o bin/diffnotes-linux-amd64 ./cmd/diffnotes

test:
	go test ./...

vet:
	go vet ./...

clean:
	rm -rf bin
