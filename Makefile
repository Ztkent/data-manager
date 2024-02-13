.PHONY: app-up
app-up:
	docker-compose -p data-manager --profile app up

.PHONY: db-up
db-up:
	docker-compose -p data-manager-db-only --profile db up

.PHONY: app-down
app-down:
	docker-compose -p data-manager --profile app down

.PHONY: db-down
db-down:
	docker-compose -p data-manager-db-only --profile db down