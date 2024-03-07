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
	npm install font-awesome@4.7.0
	npm install jquery@3.7.1
	npm install -D tailwindcss@3.4.1
	npm install -D daisyui@latest

build:
	curl -L -o public/js/htmx.min.js https://unpkg.com/htmx.org@1.9.10/dist/htmx.min.js
	cp ./node_modules/jquery/dist/jquery.min.js public/js/jquery.min.js
	cp ./node_modules/font-awesome/css/font-awesome.min.css public/css/font-awesome.min.css
	cp ./node_modules/font-awesome/fonts/* public/fonts/
	npx tailwindcss -i pkg/view/css/app.css -o public/css/styles.css
	templ generate view
	go build -v -o ./bin/wits ./cmd/server/main.go

clean:
	rm -f ./bin/wits
	rm -f coverage.html
	rm -f coverage.out
	rm -rf log
	rm -rf node_modules
	rm -rf tmp
	rm -rf vendor

local-db-up:
	kubectl apply -f k8s/

local-db-down:
	kubectl delete -f k8s/

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

seed:
	go run cmd/seed/main.go