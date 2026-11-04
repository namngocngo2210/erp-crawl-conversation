.PHONY: build test lint run migrate-up
build:
	go build -o bin/crawler ./cmd/crawler
test:
	go test ./... -race -count=1
lint:
	golangci-lint run
run:
	go run ./cmd/crawler
migrate-up:
	goose -dir db/migrations postgres "$$DATABASE_URL" up
