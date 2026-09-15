IMAGE ?= incubrix-backend
PORT ?= 8080

.PHONY: run build test tidy docker-build docker-run

run:
	go run ./cmd/server

build:
	go build -o bin/server ./cmd/server

test:
	go test ./...

tidy:
	go mod tidy

docker-build:
	docker build -t $(IMAGE) .

# --rm cleans up on exit; -e passes the same settings the binary reads locally.
docker-run:
	docker run --rm -p $(PORT):8080 \
		-e CORS_ALLOWED_ORIGINS=$(CORS_ALLOWED_ORIGINS) \
		$(IMAGE)
