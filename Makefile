.PHONY: run build clean docker-up docker-down migrate

# Build the binary
build:
	go build -tags go_json -o bin/server ./cmd/server

# Run in development mode
run:
	go run -tags go_json ./cmd/server

# Clean build artifacts
clean:
	rm -rf bin/

# Start Docker services (PostgreSQL + Redis)
docker-up:
	docker compose up -d

# Stop Docker services
docker-down:
	docker compose down

# Run migrations up
migrate-up:
	go run cmd/migrate/main.go up

# Run migrations down
migrate-down:
	go run cmd/migrate/main.go down

# Show migration version
migrate-version:
	go run cmd/migrate/main.go version

# Force migration version (Usage: make migrate-force VERSION=1)
migrate-force:
	go run cmd/migrate/main.go force $(VERSION)

# Run go vet
vet:
	go vet ./...

# Tidy dependencies
tidy:
	go mod tidy
