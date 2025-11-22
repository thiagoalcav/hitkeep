all: run

build: frontend-build go-build

go-build:
	@echo "Building Go application..."
	CGO_ENABLED=1 go build -ldflags="-w -s -X 'hitkeep/cmd.Version=snapshot'" -o hitkeep ./cmd/hitkeep/main.go

frontend-build: frontend-tracker-build frontend-dashboard-build

frontend-tracker-build:
	@echo "Building tracker snippet..."
	@cd frontend/tracker && npm install && npm run build

frontend-dashboard-build:
	@echo "Building Angular dashboard..."
	@cd frontend/dashboard && npm install && npm run build:prod
	@echo "Copying dashboard to public directory..."
	@cp -r frontend/dashboard/dist/dashboard/browser/* public/

frontend-dev:
	@echo "Starting Angular development server..."
	@cd frontend/dashboard && npm install && npm start

run: build
	@./hitkeep

clean:
	@echo "Cleaning up..."
	@rm -f ./hitkeep
	@rm -rf public
	@rm -rf frontend/tracker/dist frontend/tracker/node_modules
	@rm -rf frontend/dashboard/dist frontend/dashboard/node_modules

build-docker:
		docker buildx build . \
			--platform linux/amd64 \
			--platform linux/arm64 \
			--tag ghcr.io/pascalebeier/hitkeep:snapshot \

.PHONY: all build go-build frontend-build frontend-tracker-build frontend-dashboard-build run clean
