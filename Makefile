.PHONY: run build test swagger docker-up docker-down migrate seed seed-remote

run:
	go run ./cmd/server

build:
	go build -o bin/server ./cmd/server

test:
	go test ./...

swagger:
	swag init -g cmd/server/main.go -o docs --parseDependency --parseInternal

docker-up:
	docker compose up -d --build

docker-down:
	docker compose down

migrate:
	psql "host=$${DB_HOST:-localhost} port=$${DB_PORT:-5432} user=$${DB_USER:-postgres} password=$${DB_PASSWORD:-postgres} dbname=$${DB_NAME:-event_entry} sslmode=$${DB_SSLMODE:-disable}" -f migrations/001_initial_schema.up.sql

seed:
	go run ./cmd/seed

seed-remote:
	@./scripts/seed_remote.sh

reset:
	go run ./cmd/reset

reset-remote:
	@chmod +x scripts/reset_remote.sh scripts/reset_and_seed_remote.sh scripts/list_qrs.sh
	@./scripts/reset_remote.sh

reset-and-seed-remote:
	@chmod +x scripts/reset_remote.sh scripts/reset_and_seed_remote.sh scripts/list_qrs.sh
	@./scripts/reset_and_seed_remote.sh

list-qrs:
	@chmod +x scripts/list_qrs.sh
	@./scripts/list_qrs.sh
