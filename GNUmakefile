default: build

# Build binary
build:
	go build -o terraform-provider-cubepath

# Run unit tests
test:
	go test -v ./...

# Run acceptance tests
testacc:
	TF_ACC=1 go test -v ./internal/provider -timeout 120m

# Install provider locally
install: build
	mkdir -p ~/.terraform.d/plugins/registry.terraform.io/cubepath/cubepath/0.1.0/darwin_arm64
	cp terraform-provider-cubepath ~/.terraform.d/plugins/registry.terraform.io/cubepath/cubepath/0.1.0/darwin_arm64/

# Generate documentation
docs:
	go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs generate

# Format code
fmt:
	go fmt ./...

# Clean build artifacts
clean:
	rm -f terraform-provider-cubepath
	rm -rf .terraform
	rm -f terraform.tfstate*

# Run linter
lint:
	golangci-lint run

.PHONY: build test testacc install docs fmt clean lint
