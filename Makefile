ifneq (,$(wildcard .env))
include .env
export
endif

DATABASE_URL ?= postgres://yadori:yadori@localhost:5432/yadori?sslmode=disable
ALLOWED_ORIGINS ?= http://localhost:5173,http://localhost:5174
export DATABASE_URL
export ALLOWED_ORIGINS

.PHONY: help setup dev api worker admin booker migrate seed db-up db-down db-reset test test-short sqlc build fmt

help:
	@echo "make setup    依存をそろえる"
	@echo "make dev      API(:8080) ワーカー 管理画面(:5173) 予約者向けサイト(:5174) をまとめて起動"
	@echo "make api      APIサーバーだけ起動"
	@echo "make worker   期限切れ回収ワーカーだけ起動"
	@echo "make collect  期限切れ回収を1回だけ実行"
	@echo "make admin    管理画面だけ起動"
	@echo "make booker   予約者向けサイトだけ起動"
	@echo "make migrate  マイグレーションを適用"
	@echo "make seed     動作確認用のデモデータを入れる"
	@echo "make db-up    PostgreSQL を起動（Docker が要る）"
	@echo "make test     全テスト（DBを含む）とフロントのビルド検査"
	@echo "make test-short DB不要な高速テストのみ"
	@echo "make sqlc     SQLからコードを生成"

setup:
	cd server && go mod download
	npm --prefix web/admin install
	npm --prefix web/booker install

dev:
	@trap 'kill 0' EXIT INT TERM; \
	$(MAKE) --no-print-directory api & \
	$(MAKE) --no-print-directory worker & \
	$(MAKE) --no-print-directory admin & \
	$(MAKE) --no-print-directory booker & \
	wait

api:
	cd server && go run ./cmd/api

worker:
	cd server && go run ./cmd/worker

collect:
	cd server && go run ./cmd/worker -once

admin:
	npm --prefix web/admin run dev

booker:
	npm --prefix web/booker run dev

migrate:
	cd server && go run ./cmd/migrate up

seed:
	cd server && go run ./cmd/seed

db-up:
	docker compose up -d
	@echo "PostgreSQL が立ち上がるまで待っています..."
	@until docker compose exec -T db pg_isready -U yadori >/dev/null 2>&1; do sleep 1; done
	@echo "起動しました"

db-down:
	docker compose down

db-reset:
	docker compose down -v
	$(MAKE) db-up
	$(MAKE) migrate
	$(MAKE) seed

test:
	cd server && go test -race -count=1 ./...
	npm --prefix web/admin run build
	npm --prefix web/booker run build

test-short:
	cd server && go test -short -race -count=1 ./...

sqlc:
	cd server && go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.30.0 generate

build:
	cd server && go build -o bin/api ./cmd/api
	cd server && go build -o bin/worker ./cmd/worker
	npm --prefix web/admin run build
	npm --prefix web/booker run build

fmt:
	cd server && go fmt ./...
