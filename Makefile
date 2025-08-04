.PHONY: clean test

grpchealth: go.* *.go ./cmd/*/*.go
	go build -o $@ ./cmd/grpchealth

clean:
	rm -rf grpchealth dist/

test:
	go test -v ./...

install:
	go install github.com/fujiwara/grpchealth/cmd/grpchealth

dist:
	goreleaser build --snapshot --clean
