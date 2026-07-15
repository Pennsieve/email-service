.PHONY: help clean test test-ci package publish tidy generate

LAMBDA_BUCKET ?= "pennsieve-cc-lambda-functions-use1"
WORKING_DIR   ?= "$(shell pwd)"
SERVICE_NAME  ?= "email-service"
PACKAGE_NAME        ?= "${SERVICE_NAME}-${IMAGE_TAG}.zip"
BOUNCE_PACKAGE_NAME ?= "${SERVICE_NAME}-bounce-${IMAGE_TAG}.zip"

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

# Build both lambdas (queue + bounce) into per-lambda ZIPs. Each is a separate
# bootstrap binary zipped under its own name.
package:
	@echo ""
	@echo "*****************************"
	@echo "*   Building Queue lambda   *"
	@echo "*****************************"
	@echo ""
	env GOOS=linux GOARCH=arm64 go build -tags lambda.norpc -o $(WORKING_DIR)/bin/queue/bootstrap ./cmd/queue
	cd $(WORKING_DIR)/bin/queue && zip -r $(WORKING_DIR)/bin/$(PACKAGE_NAME) bootstrap
	@echo ""
	@echo "******************************"
	@echo "*   Building Bounce lambda   *"
	@echo "******************************"
	@echo ""
	env GOOS=linux GOARCH=arm64 go build -tags lambda.norpc -o $(WORKING_DIR)/bin/bounce/bootstrap ./cmd/bounce
	cd $(WORKING_DIR)/bin/bounce && zip -r $(WORKING_DIR)/bin/$(BOUNCE_PACKAGE_NAME) bootstrap

# Publish both lambda ZIPs to the S3 location the Terraform lambdas read from.
publish: package
	@echo ""
	@echo "***********************************"
	@echo "*   Publishing Queue + Bounce     *"
	@echo "***********************************"
	@echo ""
	aws s3 cp $(WORKING_DIR)/bin/$(PACKAGE_NAME) s3://$(LAMBDA_BUCKET)/$(SERVICE_NAME)/
	aws s3 cp $(WORKING_DIR)/bin/$(BOUNCE_PACKAGE_NAME) s3://$(LAMBDA_BUCKET)/$(SERVICE_NAME)/
	rm -rf $(WORKING_DIR)/bin

# Run go mod tidy
tidy:
	go mod tidy

# Regenerate the client builders (Go + Scala) from contract/template-variables.json.
generate:
	go run internal/gen/main.go
