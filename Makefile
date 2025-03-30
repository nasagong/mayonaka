.PHONY: proto proto_chat setup build run test clean dev infra-up infra-down logs-redis logs-app docker-dev docker-build

proto: proto_chat proto_auth

proto_chat:
	protoc --go_out=./internal/pb --go_opt=paths=source_relative \
		--go-grpc_out=./internal/pb --go-grpc_opt=paths=source_relative \
		--proto_path=./proto \
		./proto/chat/chat.proto

setup:
	go mod tidy

build:
	go build -o bin/mayonaka

infra-up:
	docker-compose up -d redis

infra-down:
	docker-compose down

dev: infra-up
	go run ./cmd/main.go

docker-build:
	docker-compose build app

docker-dev: docker-build
	docker-compose up -d
	@echo "App and Redis are starting..."
	@echo "Services are ready!"
	docker-compose logs -f app

logs-redis:
	docker-compose logs -f redis

logs-app:
	docker-compose logs -f app
