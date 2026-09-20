.PHONY: test vet build docker

test:
	go test ./...

vet:
	go vet ./...

build:
	go build ./cmd/tunescout

docker:
	docker build -t tunescout:local .
