.PHONY: help clean test test-ci package publish tidy generate migrate-up migrate-down

LAMBDA_BUCKET ?= "pennsieve-cc-lambda-functions-use1"
WORKING_DIR   ?= "$(shell pwd)"
SERVICE_NAME  ?= "email-service"
PACKAGE_NAME  ?= "${SERVICE_NAME}-${IMAGE_TAG}.zip"

# DSN for the notifications database, e.g.
# postgres://user:pass@host:5432/notifications?sslmode=require
NOTIFICATIONS_DATABASE_URL ?=
NOTIFICATIONS_MIGRATIONS_PATH ?= internal/notifications/migrations

.DEFAULT: help

help:
	@echo "Make Help for $(SERVICE_NAME)"
	@echo ""
	@echo "make clean			- spin down containers and remove build artifacts"
	@echo "make test			- run dockerized tests locally"
	@echo "make test-ci			- run dockerized tests for Jenkins"
	@echo "make package			- build and package the queue lambda"
	@echo "make publish			- package and publish the queue lambda to S3"
	@echo "make generate			- regenerate client builders from the template manifest"
	@echo "make migrate-up			- apply notifications DB migrations (needs NOTIFICATIONS_DATABASE_URL)"
	@echo "make migrate-down		- roll back one notifications DB migration (needs NOTIFICATIONS_DATABASE_URL)"

# Run dockerized tests (can be used locally)
test:
	docker-compose -f docker-compose.test.yml down --remove-orphans
	docker-compose -f docker-compose.test.yml up --exit-code-from local_tests local_tests
	make clean

# Run dockerized tests (used on Jenkins)
test-ci:
	docker-compose -f docker-compose.test.yml down --remove-orphans
	@IMAGE_TAG=$(IMAGE_TAG) docker-compose -f docker-compose.test.yml up --exit-code-from=ci-tests ci-tests

# Remove build artifacts and spin down docker containers.
clean: docker-clean
	rm -rf $(WORKING_DIR)/bin

# Spin down active docker containers.
docker-clean:
	docker-compose -f docker-compose.test.yml down

# Build the queue lambda and create the deployment ZIP.
package:
	@echo ""
	@echo "*****************************"
	@echo "*   Building Queue lambda   *"
	@echo "*****************************"
	@echo ""
	env GOOS=linux GOARCH=arm64 go build -tags lambda.norpc -o $(WORKING_DIR)/bin/bootstrap ./cmd/queue
	cd $(WORKING_DIR)/bin && zip -r $(WORKING_DIR)/bin/$(PACKAGE_NAME) bootstrap

# Publish the queue lambda ZIP to the S3 location the Terraform lambda reads from.
publish: package
	@echo ""
	@echo "*******************************"
	@echo "*   Publishing Queue lambda   *"
	@echo "*******************************"
	@echo ""
	aws s3 cp $(WORKING_DIR)/bin/$(PACKAGE_NAME) s3://$(LAMBDA_BUCKET)/$(SERVICE_NAME)/
	rm -rf $(WORKING_DIR)/bin/$(PACKAGE_NAME) $(WORKING_DIR)/bin/bootstrap

# Run go mod tidy
tidy:
	go mod tidy

# Regenerate the client builders (Go + Scala) from contract/template-variables.json.
generate:
	go run internal/gen/main.go

# Apply/roll back the notifications DB schema (internal/notifications/migrations)
# using the golang-migrate CLI (https://github.com/golang-migrate/migrate).
# Install it with: go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
migrate-up:
	migrate -path $(NOTIFICATIONS_MIGRATIONS_PATH) -database "$(NOTIFICATIONS_DATABASE_URL)" up

migrate-down:
	migrate -path $(NOTIFICATIONS_MIGRATIONS_PATH) -database "$(NOTIFICATIONS_DATABASE_URL)" down 1
