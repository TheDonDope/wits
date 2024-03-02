# Wits - The Weed Information Tracking System

[![codecov](https://codecov.io/gh/TheDonDope/wits/graph/badge.svg?token=QM1XTAUsfU)](https://codecov.io/gh/TheDonDope/wits)

Wits aims to help cannabis patients and users to manage and monitor their cannabis consumption and inventory.

## Building the Application

Building the binary requires multiple steps:

- Compiling the Tailwind CSS output
- Generating the Go code from the Templ Templates
- Building the Go binary

To do this in one command, run the following (alternatively `$ task build` if you are using [Task](https://taskfile.dev/#/)):

```shell
$ make build
curl -L -o public/js/htmx.min.js https://unpkg.com/htmx.org@1.9.10/dist/htmx.min.js
  % Total    % Received % Xferd  Average Speed   Time    Time     Time  Current
                                 Dload  Upload   Total   Spent    Left  Speed
100 47755    0 47755    0     0   216k      0 --:--:-- --:--:-- --:--:--  217k
cp ./node_modules/jquery/dist/jquery.min.js public/js/jquery.min.js
cp ./node_modules/font-awesome/css/font-awesome.min.css public/css/font-awesome.min.css
cp ./node_modules/font-awesome/fonts/* public/fonts/
npx tailwindcss -i pkg/view/css/app.css -o public/css/styles.css

Rebuilding...

🌼   daisyUI 4.7.2
├─ ✔︎ 2 themes added  https://daisyui.com/docs/themes
╰─ ❤︎ Support daisyUI project: https://opencollective.com/daisyui


Done in 200ms.
templ generate view
(✓) Complete [ updates=4 duration=23.351583ms ]
go build -v -o ./bin/wits ./cmd/server.go
```

## Running the Application

### Required Environment Variables

The following environment variables are required to run the application:

| Environment Variable      | Description                                                                                         |
| ------------------------- | --------------------------------------------------------------------------------------------------- |
| `HTTP_LISTEN_ADDR`        | The address the server runs at (format: `<url>:<port>`, example: `:3000`)                           |
| `JWT_SECRET_KEY`          | The secret key with which to sign the Access Token                                                  |
| `JWT_REFRESH_SECRET_KEY`  | The secret key with which to sign the Refresh Token                                                 |
| `DB_TYPE`                 | The type of database to use (choose `local` for local Sqlite db or `remote` for remote Supabase db) |
| `SQLITE_DATA_SOURCE_NAME` | The name of the Sqlite datasource to use (example: `./bin/wits.db`), when `DB_TYPE=local`           |
| `DB_HOST`                 | The host of the postgres db, when `DB_TYPE=remote`                                                  |
| `DB_USER`                 | The user of the postgres db, when `DB_TYPE=remote`                                                  |
| `DB_PASSWORD`             | The password of the postgres db, when `DB_TYPE=remote`                                              |
| `DB_NAME`                 | The name of the postgres db, when `DB_TYPE=remote`                                                  |
| `SUPABASE_URL`            | The Supabase URL (required for the client configuration),, when `DB_TYPE=remote`                    |
| `SUPABASE_SECRET`         | The Supabase secret (required for the client configuration),, when `DB_TYPE=remote`                 |

The built application binary can be started by:

```shell
$ ./bin/wits
2024/03/02 00:21:11 INFO 🥦 🖥️  Welcome to Wits!
2024/03/02 00:21:11 INFO 📁 🏠 Using local sqlite database with dsn=./bin/wits.db
2024/03/02 00:21:11 INFO 🚀 🖥️  Wits server is running at addr=:3000

   ____    __
  / __/___/ /  ___
 / _// __/ _ \/ _ \
/___/\__/_//_/\___/ v4.11.4
High performance, minimalist Go web framework
https://echo.labstack.com
____________________________________O/_______
                                    O\
⇨ http server started on [::]:3000
```

The built binary is explicitly ignored from source control (see [.gitignore](.gitignore)).

## Running Tests

- Run the testsuite with coverage enabled (alternatively `$ task test` if you are using [Task](https://taskfile.dev/#/)):

```shell
$ make test
go test -race -v ./... -coverprofile coverage.out
?    github.com/TheDonDope/wits/pkg/types [no test files]
?    github.com/TheDonDope/wits/pkg/view [no test files]
 github.com/TheDonDope/wits/pkg/handler  coverage: 0.0% of statements
 github.com/TheDonDope/wits/cmd  coverage: 0.0% of statements
 github.com/TheDonDope/wits/pkg/view/dashboard  coverage: 0.0% of statements
 github.com/TheDonDope/wits/pkg/view/layout  coverage: 0.0% of statements
 github.com/TheDonDope/wits/pkg/view/auth  coverage: 0.0% of statements
 github.com/TheDonDope/wits/pkg/storage  coverage: 0.0% of statements
 github.com/TheDonDope/wits/pkg/view/ui  coverage: 0.0% of statements
```

- Generate the coverage results as html (alternatively `$ task cover` if you are using [Task](https://taskfile.dev/#/)):

```shell
$ make cover
go test -race -v ./... -coverprofile coverage.out
?    github.com/TheDonDope/wits/pkg/types [no test files]
?    github.com/TheDonDope/wits/pkg/view [no test files]
 github.com/TheDonDope/wits/cmd  coverage: 0.0% of statements
 github.com/TheDonDope/wits/pkg/handler  coverage: 0.0% of statements
 github.com/TheDonDope/wits/pkg/view/auth  coverage: 0.0% of statements
 github.com/TheDonDope/wits/pkg/view/ui  coverage: 0.0% of statements
 github.com/TheDonDope/wits/pkg/view/layout  coverage: 0.0% of statements
 github.com/TheDonDope/wits/pkg/storage  coverage: 0.0% of statements
 github.com/TheDonDope/wits/pkg/view/dashboard  coverage: 0.0% of statements
go tool cover -html coverage.out -o coverage.html
[Empty output on success]
```

- Open the results in the browser (alternatively `$ task show-cover` if you are using [Task](https://taskfile.dev/#/)):

```shell
$ make show-cover
go test -race -v ./... -coverprofile coverage.out
?    github.com/TheDonDope/wits/pkg/types [no test files]
?    github.com/TheDonDope/wits/pkg/view [no test files]
 github.com/TheDonDope/wits/cmd  coverage: 0.0% of statements
 github.com/TheDonDope/wits/pkg/handler  coverage: 0.0% of statements
 github.com/TheDonDope/wits/pkg/storage  coverage: 0.0% of statements
 github.com/TheDonDope/wits/pkg/view/ui  coverage: 0.0% of statements
 github.com/TheDonDope/wits/pkg/view/auth  coverage: 0.0% of statements
 github.com/TheDonDope/wits/pkg/view/layout  coverage: 0.0% of statements
 github.com/TheDonDope/wits/pkg/view/dashboard  coverage: 0.0% of statements
go tool cover -html coverage.out -o coverage.html
open coverage.html
<Opens Browser>
```

Both the `coverage.out` as well as the `coverage.html` are explicitly ignored from source control (see [.gitignore](.gitignore)).
