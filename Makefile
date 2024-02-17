.PHONY: app-up
app-up:
	docker-compose -p data-manager --profile data-manager up

.PHONY: db-up
db-up:
	docker-compose -p data-manager-db-only --profile db up

.PHONY: app-down
app-down:
	docker-compose -p data-manager --profile data-manager down

.PHONY: db-down
db-down:
	docker-compose -p data-manager-db-only --profile db down