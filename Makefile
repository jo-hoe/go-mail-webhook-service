include help.mk

# get root dir
ROOT_DIR := $(dir $(realpath $(lastword $(MAKEFILE_LIST))))
IMAGE_NAME := go-mail-webhook-service
IMAGE_TAG := mws

.DEFAULT_GOAL := start

DOCKER_COMPOSE_CMD := docker compose up

.PHONY: update
update: ## update dependencies
	git pull
	go mod tidy

.PHONY: lint
lint: ## run linters
	golangci-lint run

.PHONY: start
start: ## start via docker
	@${DOCKER_COMPOSE_CMD}

.PHONY: start-rebuild
start-rebuild: ## rebuild and start via docker
	@${DOCKER_COMPOSE_CMD} --build

.PHONY: dev-build-container
dev-build-container: ## builds container
	@docker build . -t ${IMAGE_TAG}

DOCKER_START_CMD := @docker run --rm -v "${ROOT_DIR}/config/:/go/config"

.PHONY: dev-exec-container
dev-exec-container: dev-build-container ## builds container and execs into it
	${DOCKER_START_CMD} -it --entrypoint /bin/sh ${IMAGE_TAG} 

.PHONY: dev-build-start
dev-build-start: dev-build-container ## builds and starts the container
	${DOCKER_START_CMD} ${IMAGE_TAG}
