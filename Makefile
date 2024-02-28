run: build
	@./bin/wits

install:
	go install github.com/a-h/templ/cmd/templ@latest
	go install github.com/cosmtrek/air@latest
	go install github.com/cweill/gotests/gotests@latest
	go install golang.org/x/tools/cmd/godoc@latest

	go get ./...
	go mod vendor
	go mod tidy
	go mod download
	npm install -D tailwindcss
	npm install -D daisyui@latest

build:
	npx tailwindcss -i pkg/view/css/app.css -o public/css/styles.css
	templ generate view
	go build -v -o ./bin/wits ./cmd/server.go

clean:
	rm -rf ./bin
	rm -f coverage.html
	rm -f coverage.out

doc:
	godoc

test:
	go test -race -v ./... -coverprofile coverage.out

test-ci:
	go test -race -v ./... -coverprofile coverage.out -covermode=atomic
	bash -c "bash <(curl -s https://codecov.io/bash)"

cover: test
	go tool cover -html coverage.out -o coverage.html

show-cover: cover
	open coverage.html

vet:
	go vet ./...

up: ## Database migration up
	go run cmd/migrate/main.go up

drop:
	go run cmd/drop/main.go up

down: ## Database migration down
	go run cmd/migrate/main.go down

migration: ## Migrations against the database
	migrate create -ext sql -dir cmd/migrate/migrations $(filter-out $@,$(MAKECMDGOALS))

gen:
	go run cmd/generate/main.go

seed:
	go run cmd/seed/main.go