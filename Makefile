# Variables needed for rest of Makefile
# IMAGE_REPO ?= e.g. aws-observability
# IMAGE_NAME ?= e.g. aws-sigv4-admission-controller
# IMAGE_TAG ?=  e.g v1

all: build-image push-image

build-image:
	@echo "Building the docker image: $(IMAGE_REPO)/$(IMAGE_NAME):$(IMAGE_TAG)..."
	@docker build -t $(IMAGE_REPO)/$(IMAGE_NAME):$(IMAGE_TAG) .

push-image:
	@echo "Pushing the docker image for $(IMAGE_REPO)/$(IMAGE_NAME):$(IMAGE_TAG)..."
	@docker push $(IMAGE_REPO)/$(IMAGE_NAME):$(IMAGE_TAG)

test:
	@go test -v ./...
