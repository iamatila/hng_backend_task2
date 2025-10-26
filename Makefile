.PHONY: run build install clean test refresh status

# Run the application
run:
	go run main.go

# Build the application
build:
	go build -o bin/country-api main.go

# Install dependencies
install:
	go mod download
	go mod tidy

# Clean build artifacts
clean:
	rm -rf bin/
	rm -rf cache/

# Run tests
test:
	go test -v ./...

# Setup database (requires MySQL running)
setup-db:
	mysql -u root -p -e "CREATE DATABASE IF NOT EXISTS countries_db CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;"

# Quick refresh (call the API)
refresh:
	curl -X POST http://localhost:3000/countries/refresh

# Get status
status:
	curl http://localhost:3000/status

# Get all countries
countries:
	curl http://localhost:3000/countries

# Run in production mode
prod:
	./bin/country-api

# Development with auto-reload (requires air: go install github.com/cosmtrek/air@latest)
dev:
	air

# Format code
fmt:
	go fmt ./...

# Vet code
vet:
	go vet ./...