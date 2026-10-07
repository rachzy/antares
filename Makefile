.DEFAULT_GOAL := help

NETWORK    ?= antares
BIND       ?= 127.0.0.1
WWW_PORT   ?= 8080
SHAULA_PORT ?= 50051
FANG_PORT  ?= 50052

.PHONY: help install setup proto dev dev-www dev-shaula dev-fang build lint lint-fix \
	format format-check test ci docker-build up down restart ps logs clean \
	up-www up-shaula up-fang

help: ## Show this help
	@awk 'BEGIN {FS = ":.*## "} /^[a-zA-Z_-]+:.*## / {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

install: ## Install npm and Python dependencies
	npm install
	npm run install:py

setup: install proto ## Install everything and generate protobuf stubs

proto: ## Regenerate protobuf stubs (Go + Python)
	npm run proto:generate

dev: ## Run www, shaula and fang in dev mode
	npm run dev

dev-www: ## Run the Angular dev server
	npm run dev:www

dev-shaula: ## Run the Shaula service locally
	npm run dev:shaula

dev-fang: ## Run the Fang service locally
	npm run dev:fang

build: ## Production build of www
	npm run build

lint: ## Lint all projects
	npm run lint

lint-fix: ## Lint all projects and apply fixes
	npm run lint:fix

format: ## Format all projects
	npm run format

format-check: ## Check formatting without writing
	npm run format:check

test: ## Run all unit tests
	npm run test

ci: ## Lint, format check, test and build
	npm run ci

docker-build: ## Build the www, shaula and fang images
	npm run build:docker

$(NETWORK)-network:
	@docker network inspect $(NETWORK) >/dev/null 2>&1 || docker network create $(NETWORK) >/dev/null

up-www: $(NETWORK)-network
	@docker rm -f antares-www >/dev/null 2>&1 || true
	docker run -d --name antares-www --network $(NETWORK) --restart unless-stopped \
		-p $(BIND):$(WWW_PORT):80 antares-www:local

up-shaula: $(NETWORK)-network
	@docker rm -f antares-shaula >/dev/null 2>&1 || true
	docker run -d --name antares-shaula --network $(NETWORK) --restart unless-stopped \
		-p $(BIND):$(SHAULA_PORT):50051 \
		-v antares-shaula-cache:/var/cache/shaula/lightkurve antares-shaula:local

up-fang: $(NETWORK)-network
	@docker rm -f antares-fang >/dev/null 2>&1 || true
	docker run -d --name antares-fang --network $(NETWORK) --restart unless-stopped \
		-p $(BIND):$(FANG_PORT):50052 \
		-v antares-fang-models:/var/lib/fang/models antares-fang:local

up: up-www up-shaula up-fang ## Run all containers (images must be built)

down: ## Stop and remove all containers (volumes are kept)
	-docker rm -f antares-www antares-shaula antares-fang

restart: down up ## Recreate all containers

ps: ## List running Antares containers
	@docker ps --filter name=antares- --format 'table {{.Names}}\t{{.Status}}\t{{.Ports}}'

logs: ## Follow logs of all containers (Ctrl-C to stop)
	@for c in antares-www antares-shaula antares-fang; do \
		docker logs -f --tail 20 $$c 2>&1 | sed "s/^/[$$c] /" & \
	done; wait

clean: ## Remove local build output and service data
	rm -rf dist .nx/cache .data
