.DEFAULT_GOAL := help

BIND       ?= 127.0.0.1
WWW_PORT   ?= 8080
SHAULA_PORT ?= 50051
FANG_PORT  ?= 50052
API_PORT   ?= 8081
PROXY_PORT ?= 80

.PHONY: help install setup proto dev dev-www dev-api dev-shaula dev-fang build lint lint-fix \
	format format-check test ci docker-build up up-dev build-up fang-models down restart ps logs clean

help: ## Show this help
	@awk 'BEGIN {FS = ":.*## "} /^[a-zA-Z_-]+:.*## / {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}' $(MAKEFILE_LIST)

install: ## Install npm, Python and Go protobuf plugin dependencies
	npm install
	npm run install:py
	npm run install:go

setup: install proto ## Install everything and generate protobuf stubs

proto: ## Regenerate protobuf stubs (Go + Python)
	npm run proto:generate

dev: ## Run www, api, shaula and fang in dev mode
	npm run dev

dev-www: ## Run the Angular dev server
	npm run dev:www

dev-api: ## Run the Go API locally
	npm run dev:api

dev-shaula: ## Run the Shaula service locally
	npm run dev:shaula

dev-fang: ## Run the Fang service locally
	npm run dev:fang

build: ## Production build of www and api
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

docker-build: ## Build the www, api, shaula and fang images
	npm run build:docker

COMPOSE      = docker compose -f compose.yaml -f compose.$(ENV).yaml
ENV         ?= prod

up: ## Start the production stack (images must be built; ENV=dev for the dev stack)
	$(COMPOSE) up -d

up-dev: ## Start the dev stack with every service port published
	$(MAKE) up ENV=dev

build-up: ## Build the images and start the stack
	$(COMPOSE) up -d --build

down: ## Stop and remove the containers (volumes are kept)
	-docker compose -f compose.yaml -f compose.prod.yaml down
	-docker compose -f compose.yaml -f compose.dev.yaml down

fang-models: ## Point Fang at a model bundle directory and restart it (MODELS=/abs/path)
	@test -n "$(MODELS)" || { echo "usage: make fang-models MODELS=/absolute/path/to/bundle"; exit 1; }
	@case "$(MODELS)" in /*) ;; *) echo "MODELS must be an absolute path"; exit 1;; esac
	@test -d "$(MODELS)" || { echo "$(MODELS) is not a directory"; exit 1; }
	@touch .env && sed -i '/^FANG_MODELS_DIR=/d' .env && echo "FANG_MODELS_DIR=$(MODELS)" >> .env
	$(COMPOSE) up -d --no-deps fang

restart: ## Recreate all containers
	$(COMPOSE) up -d --force-recreate

ps: ## List the running Antares containers
	docker compose -f compose.yaml -f compose.$(ENV).yaml ps

logs: ## Follow logs of all containers (Ctrl-C to stop)
	$(COMPOSE) logs -f --tail 20

clean: ## Remove local build output and service data
	rm -rf dist .nx/cache .data
