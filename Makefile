.PHONY: app-up
app-up:
	docker-compose -p data-manager --profile app up

.PHONY: db-up
db-up:
	docker-compose -p data-manager-db-only --profile db up

.PHONY: remote-app-up
remote-app-up:
	docker-compose -p data-manager --profile remote-app up

.PHONY: app-down
app-down:
	docker-compose -p data-manager --profile app down

.PHONY: db-down
db-down:
	docker-compose -p data-manager-db-only --profile db down

.PHONY: remote-app-down
remote-app-down:
	docker-compose -p data-manager --profile remote-app down