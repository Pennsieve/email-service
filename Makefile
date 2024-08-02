.PHONY: help clean test test-ci package publish

LAMBDA_BUCKET ?= "pennsieve-cc-lambda-functions-use1"
WORKING_DIR   ?= "$(shell pwd)"
SERVICE_NAME  ?= "email-service"
PACKAGE_API_NAME  ?= "${SERVICE_NAME}-api-${IMAGE_TAG}.zip"
PACKAGE_QUEUE_NAME  ?= "${SERVICE_NAME}-queue-${IMAGE_TAG}.zip"

.DEFAULT: help

help:
	@echo "Make Help for $(SERVICE_NAME)"
	@echo ""
	@echo "make clean			- spin down containers and remove db files"
	@echo "make test			- run dockerized tests locally"
	@echo "make test-ci			- run dockerized tests for Jenkins"
	@echo "make package			- create venv and package lambda function"
	@echo "make publish			- package and publish lambda function"

# Run dockerized tests (can be used locally)
test:
	docker-compose -f docker-compose.test.yml down --remove-orphans
	docker-compose -f docker-compose.test.yml up --exit-code-from local_tests local_tests
	make clean

# Run dockerized tests (used on Jenkins)
test-ci:
	docker-compose -f docker-compose.test.yml down --remove-orphans
	@IMAGE_TAG=$(IMAGE_TAG) docker-compose -f docker-compose.test.yml up --exit-code-from=ci-tests ci-tests

# Remove folders created by NEO4J docker container
clean: docker-clean
	rm -rf conf
	rm -rf data
	rm -rf plugins

# Spin down active docker containers.
docker-clean:
	docker-compose -f docker-compose.test.yml down

# Build lambda and create ZIP file
package:
	@echo ""
	@echo "***************************"
	@echo "*   Building API lambda   *"
	@echo "**************************"
	@echo ""
	cd lambda/service; \
  		env GOOS=linux GOARCH=arm64 go build -tags lambda.norpc -o $(WORKING_DIR)/lambda/bin/service/bootstrap; \
		cd $(WORKING_DIR)/lambda/bin/service/ ; \
			zip -r $(WORKING_DIR)/lambda/bin/service/$(PACKAGE_API_NAME) .
	@echo ""
	@echo "*****************************"
	@echo "*   Building Queue lambda   *"
	@echo "*****************************"
	@echo ""
	cd lambda/queue; \
		env GOOS=linux GOARCH=arm64 go build -tags lambda.norpc -o $(WORKING_DIR)/lambda/bin/queue/bootstrap; \
		cd $(WORKING_DIR)/lambda/bin/queue/ ; \
			zip -r $(WORKING_DIR)/lambda/bin/queue/$(PACKAGE_QUEUE_NAME) .

# Copy Service lambda to S3 location
publish:
	@make package
	@echo ""
	@echo "*****************************"
	@echo "*   Publishing API lambda   *"
	@echo "*****************************"
	@echo ""
	aws s3 cp $(WORKING_DIR)/lambda/bin/service/$(PACKAGE_API_NAME) s3://$(LAMBDA_BUCKET)/$(SERVICE_NAME)/
	rm -rf $(WORKING_DIR)/lambda/bin/service/$(PACKAGE_API_NAME) $(WORKING_DIR)/lambda/bin/service/bootstrap
	@echo ""
	@echo "*******************************"
	@echo "*   Publishing Queue lambda   *"
	@echo "*******************************"
	@echo ""
	aws s3 cp $(WORKING_DIR)/lambda/bin/queue/$(PACKAGE_QUEUE_NAME) s3://$(LAMBDA_BUCKET)/$(SERVICE_NAME)/
	rm -rf $(WORKING_DIR)/lambda/bin/queue/$(PACKAGE_NAME) $(WORKING_DIR)/lambda/bin/queue/bootstrap

# Run go mod tidy on modules
tidy:
	cd ${WORKING_DIR}/lambda/service; go mod tidy
	cd ${WORKING_DIR}/lambda/queue; go mod tidy
