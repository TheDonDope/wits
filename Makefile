run: build
	@./bin/wits

install:
	go install github.com/a-h/templ/cmd/templ@latest
	go install golang.org/x/tools/cmd/godoc@latest
	go install github.com/git-chglog/git-chglog/cmd/git-chglog@latest

	npm install

build-server:
	curl -L -o public/js/htmx.min.js https://unpkg.com/htmx.org@1.9.12/dist/htmx.min.js
	cp ./node_modules/jquery/dist/jquery.min.js public/js/jquery.min.js
	cp ./node_modules/font-awesome/css/font-awesome.min.css public/css/font-awesome.min.css
	cp ./node_modules/font-awesome/fonts/* public/fonts/
	npx @tailwindcss/cli -i pkg/view/css/app.css -o public/css/styles.css
	templ generate view
	go build -v -o ./bin/wits-server ./cmd/server/main.go

build-cli:
	go build -v -o ./bin/wits ./cmd/cli/main.go

build: build-server build-cli

build-image:
	podman build -t thedondope/wits .

push-image: build-image
	podman tag thedondope/wits ghcr.io/thedondope/wits:latest
	podman push ghcr.io/thedondope/wits:latest

k8s-up:
	kubectl apply -f k8s/

k8s-down:
	kubectl delete -f k8s/

clean:
	rm -f ./bin/wits-server
	rm -f ./bin/wits
	rm -f coverage.html
	rm -f coverage.out
	rm -rf log
	rm -rf node_modules
	rm -rf tmp
	rm -rf vendor

doc:
	godoc

changelog:
	git-chglog -o CHANGELOG.md

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
