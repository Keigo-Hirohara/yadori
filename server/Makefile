.PHONY: up down migrate-up migrate-down sqlc test

up:
	docker compose up -d postgres

down:
	docker compose down

migration:
	docker compose run --rm --entrypoint migrate migrate \
		create -ext sql -dir /migrations -seq $(name)

migrate-up:
	docker compose run --rm migrate up

migrate-down:
	docker compose run --rm migrate down 1

sqlc:
	docker compose run --rm sqlc generate

test:
	go test -race -count=1 ./...

test-short:
	go test -short ./...