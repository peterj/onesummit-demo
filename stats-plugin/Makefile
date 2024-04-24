DOCKER_USER ?= pj3677
DOCKER_REGISTRY ?= docker.io/$(DOCKER_USER)
IMAGE_NAME ?= stats-plugin
IMAGE_TAG ?= 0.0.1

build: main.go go.mod
	@echo "Building wasm..."
	@go get
	@tinygo build -o main.wasm -scheduler=none -target=wasi

test:
	@echo "Running tests..."
	@go test ./...

fmt:
	gofmt -w .

clean:
	rm -f main.wasm

docker: test build
	cp main.wasm plugin.wasm
	@echo "Building docker image..."
	@docker build -t $(IMAGE_NAME):$(IMAGE_TAG) .

publish: test build docker
	@echo "Publishing docker image $(DOCKER_REGISTRY)/$(IMAGE_NAME):$(IMAGE_TAG)..."
	@docker tag $(IMAGE_NAME):$(IMAGE_TAG) $(DOCKER_REGISTRY)/$(IMAGE_NAME):$(IMAGE_TAG)
	@docker push $(DOCKER_REGISTRY)/$(IMAGE_NAME):$(IMAGE_TAG)
