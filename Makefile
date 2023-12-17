VERSION?=latest
IMAGE_NAME=ghcr.io/aokumasan/external-dns-nifcloud-webhook:$(VERSION)

.PHONY: build
build:
	CGO_ENABLED=0 go build -o bin/webhook -ldflags '-w -extldflags "-static"' cmd/webhook/main.go

.PHONY: test
test:
	go test ./...

.PHONY: image
image:
	docker build --platform linux/amd64 -t $(IMAGE_NAME) .

.PHONY: push
push:
	docker push $(IMAGE_NAME)
