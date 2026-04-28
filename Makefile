.PHONY: run build up down migrate

run:
	go run ./cmd/bot

build:
	go build -o bin/eecp-bot ./cmd/bot

up:
	docker compose up -d

down:
	docker compose down

migrate:
	go run ./cmd/bot -migrate

.DEFAULT_GOAL := run
