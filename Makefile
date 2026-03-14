all: run

build: frontend-build go-build

go-build:
	@echo "Building Go application..."
	CGO_ENABLED=1 go build -ldflags="-w -s -X 'hitkeep/cmd.Version=snapshot'" -o hitkeep ./cmd/hitkeep/main.go

frontend-build: frontend-dashboard-build

frontend-dashboard-build:
	@echo "Building Angular dashboard and tracker snippet..."
	@cd frontend/dashboard && npm ci --no-fund --no-audit && npm run build:prod
	@echo "Copying dashboard to public directory..."
	@cp -r frontend/dashboard/dist/dashboard/browser/* public/

dev:
	@echo "Starting development environment..."
	@if ! command -v air > /dev/null; then \
		echo "Air is not installed. Installing..."; \
		go install github.com/air-verse/air@latest; \
	fi
	@make -j2 dev-backend dev-frontend

dev-backend:
	@echo "Starting Backend with Live Reload..."
	@HITKEEP_JWT_SECRET=$${HITKEEP_JWT_SECRET:-hitkeep-dev-jwt-secret} air

dev-frontend:
	@echo "Starting Angular with Hot Reload..."
	@cd frontend/dashboard && npm i --no-fund --no-audit && npm start

run: build
	@./hitkeep

clean:
	@echo "Cleaning up..."
	@rm -f ./hitkeep
	@rm -rf public
	@rm -rf frontend/dashboard/dist frontend/dashboard/node_modules

build-docker:
	@echo "Building binary for local platform..."
	CGO_ENABLED=1 go build -ldflags="-w -s -X 'hitkeep/cmd.Version=snapshot'" -o hitkeep-linux-amd64 ./cmd/hitkeep/main.go
	docker buildx build . \
		--platform linux/amd64 \
		--tag ghcr.io/pascalebeier/hitkeep:snapshot \
		--load

.PHONY: all build go-build frontend-build frontend-dashboard-build run clean
