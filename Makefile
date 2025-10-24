COMPOSE ?= docker compose
PROJECT ?= chat-app
SERVICES ?=

BACKEND_SERVICES := client-server proxy-server
FRONTEND_SERVICES := frontend
DATA_SERVICES := dynamodb redis

.PHONY: help up down stop restart build rebuild logs ps clean \
	backend backend-rebuild frontend frontend-rebuild data data-rebuild

help:
	@echo "Chat App make targets:"
	@echo "  make up [SERVICES=\"svc1 svc2\"]    - Start all (or selected) services"
	@echo "  make down                           - Stop and remove stack"
	@echo "  make restart [SERVICES=...]         - Recreate containers with latest images"
	@echo "  make build [SERVICES=...]           - Build images"
	@echo "  make rebuild [SERVICES=...]         - Force rebuild without cache"
	@echo "  make logs [SERVICES=...]            - Follow service logs"
	@echo "  make clean                          - Full teardown with volumes"
	@echo "  make backend                        - Restart API + proxy services"
	@echo "  make backend-rebuild                - Force rebuild API + proxy"
	@echo "  make frontend / frontend-rebuild    - Manage frontend service"
	@echo "  make data / data-rebuild            - Manage DynamoDB + Redis"

up:
	$(COMPOSE) -p $(PROJECT) up -d $(SERVICES)
	if [ -z "$(SERVICES)" ] || echo "$(SERVICES)" | grep -Eq '(^|[[:space:]])dynamodb([[:space:]]|$$)'; then \
		COMPOSE_PROJECT_NAME=$(PROJECT) ./scripts/fix-dynamodb-permissions.sh; \
	fi

down:
	$(COMPOSE) -p $(PROJECT) down

stop:
	$(COMPOSE) -p $(PROJECT) stop $(SERVICES)

restart:
	$(COMPOSE) -p $(PROJECT) up -d --build --force-recreate $(SERVICES)

build:
	$(COMPOSE) -p $(PROJECT) build $(SERVICES)

rebuild:
	$(COMPOSE) -p $(PROJECT) build --no-cache $(SERVICES)

logs:
	$(COMPOSE) -p $(PROJECT) logs -f $(SERVICES)

ps:
	$(COMPOSE) -p $(PROJECT) ps

clean:
	$(COMPOSE) -p $(PROJECT) down -v --remove-orphans

backend:
	$(MAKE) SERVICES="$(BACKEND_SERVICES)" restart

backend-rebuild:
	$(MAKE) SERVICES="$(BACKEND_SERVICES)" rebuild
	$(MAKE) SERVICES="$(BACKEND_SERVICES)" up

frontend:
	$(MAKE) SERVICES="$(FRONTEND_SERVICES)" restart

frontend-rebuild:
	$(MAKE) SERVICES="$(FRONTEND_SERVICES)" rebuild
	$(MAKE) SERVICES="$(FRONTEND_SERVICES)" up

data:
	$(MAKE) SERVICES="$(DATA_SERVICES)" up

data-rebuild:
	$(MAKE) SERVICES="$(DATA_SERVICES)" rebuild
	$(MAKE) SERVICES="$(DATA_SERVICES)" up
