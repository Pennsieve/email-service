.PHONY: help clean test test-ci package publish tidy

LAMBDA_BUCKET ?= "pennsieve-cc-lambda-functions-use1"
WORKING_DIR   ?= "$(shell pwd)"
SERVICE_NAME  ?= "email-service"
PACKAGE_NAME  ?= "${SERVICE_NAME}-${IMAGE_TAG}.zip"

.DEFAULT: help

help:
	@echo "Make Help for $(SERVICE_NAME)"
	@echo ""
	@echo "make clean			- spin down containers and remove build artifacts"
	@echo "make test			- run dockerized tests locally"
	@echo "make test-ci			- run dockerized tests for Jenkins"
	@echo "make package			- build and package the queue lambda"
	@echo "make publish			- package and publish the queue lambda to S3"

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
