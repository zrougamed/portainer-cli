.PHONY: up down restart logs token clean ps open help

# Default target
help:
	@echo ""
	@echo "  ⚓ Portainer CE — Make Targets"
	@echo ""
	@echo "  make up       Start Portainer CE"
	@echo "  make down     Stop and remove containers"
	@echo "  make restart  Restart Portainer"
	@echo "  make logs     Tail Portainer logs"
	@echo "  make token    Fetch JWT + API token (after 'make up')"
	@echo "  make open     Open Portainer in browser"
	@echo "  make ps       Show running containers"
	@echo "  make clean    Remove containers, volumes and data"
	@echo ""

up:
	@echo "⚓ Starting Portainer CE..."
	docker compose up -d
	@echo "✓ Portainer started. Waiting for health check..."
	@sleep 3
	@docker compose ps

down:
	docker compose down

restart:
	docker compose restart portainer

logs:
	docker compose logs -f portainer

token:
	@bash scripts/get-token.sh

ps:
	docker compose ps

open:
	@echo "Opening http://localhost:9000 ..."
	@command -v xdg-open >/dev/null 2>&1 && xdg-open http://localhost:9000 || \
	 command -v open      >/dev/null 2>&1 && open      http://localhost:9000 || \
	 echo "Visit http://localhost:9000"

clean:
	@echo "⚠ This will remove all Portainer data!"
	@read -p "Are you sure? [y/N] " confirm && [ "$$confirm" = "y" ]
	docker compose down -v
	@echo "✓ Cleaned up."