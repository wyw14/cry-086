.PHONY: fmt build test race vet web-install web-test web-build verify run simulator docker-up docker-down

fmt:
	gofmt -w $$(find cmd internal tests -name '*.go')

build:
	go build ./...

test:
	go test ./...

race:
	go test -race ./...

vet:
	go vet ./...

web-install:
	cd web && npm ci

web-test:
	cd web && npm test

web-build:
	cd web && npm run build

verify: build test race vet web-test web-build

run:
	go run ./cmd/server

simulator:
	go run ./cmd/simulator

docker-up:
	docker compose up --build

docker-down:
	docker compose down -v
