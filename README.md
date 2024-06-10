# Wits - The 🥦 Information Tracking System

[![codecov](https://codecov.io/gh/TheDonDope/wits/graph/badge.svg?token=QM1XTAUsfU)](https://codecov.io/gh/TheDonDope/wits)

Wits aims to help cannabis patients and users to manage and monitor their cannabis consumption and inventory.

## Building the Application

Building the binary requires multiple steps:

- Compiling the Tailwind CSS output
- Generating the Go code from the Templ Templates
- Building the Go binary

To do this in one command, run the following:

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

## Building and pushing the container image

Assuming the use of [Podman](https://podman.io/), the following steps are supported:

### Building the latest local image

You can run `make build-image`.

### Pushing the latest tag to GitHub Container Registry

_Prerequisite_: Acquire a Personal Access Token (PAT) in your GitHub Account.

Then, login with GHCR, like so:

```bash
$ echo $PAT | podman login ghcr.io --username <YOUR_GITHUB_USERNAME> --password-stdin
Login Succeeded!
```

Now, the latest local image can be pushed by running `make push-image`.

## Running the Application

Wits can be run in two different flavours via environment variables, either `DB_TYPE=local` or `DB_TYPE=remote`. In the applications context, `local` means:

- The application handles User login and registration itself
- JWT tokens are self-signed and stored in an encrypted session cookie (using [gorilla/sessions](https://github.com/gorilla/sessions))
- User data is stored in a Postgres database (the connection is configurable with environment variables, see below)
- Domain data is stored in a Postgres database (the connection is configurable with environment variables, see below)

In contrast, `remote` means:

- The application uses Supabase for User login and registration (including managing and storage of user data)
- The user can also login with their Google account
- JWT tokens are signed by Supabase and stored in an encrypted session cookie (using [gorilla/sessions](https://github.com/gorilla/sessions))
- Domain data is stored in a Postgres database, hosted on Supabase (the connection is configurable with environment variables, see below)

### Required Environment Variables

A minimum viable `.env` file can be found at [.env.example](.env.example). Simply rename it to `.env` to be able to run the application with a Postgres database. Fill in the other values if you want to integrate with Supabase.

The following environment variables are required to run the application:

| Environment Variable     | Description                                                                                                                                   |
| ------------------------ | --------------------------------------------------------------------------------------------------------------------------------------------- |
| `HTTP_LISTEN_ADDR`       | The address the server runs at (format: `<url>:<port>`, example: `127.0.0.1:3000`)                                                            |
| `LOG_LEVEL`              | The level at which to log (one of: `DEBUG`, `INFO`, `WARN`, `ERROR`, `OFF`)                                                                   |
| `LOG_DIR`                | The path to the directory for the application logs                                                                                            |
| `LOG_FILE`               | The name of the file for the application logs (within `LOG_DIR`)                                                                              |
| `ACCESS_LOG_FILE`        | The path of the file for the application access logs (within `LOG_DIR`)                                                                       |
| `JWT_SECRET_KEY`         | The secret key with which to sign the access token (only relevant for `DB_TYPE=local`)                                                        |
| `JWT_REFRESH_SECRET_KEY` | The secret key with which to sign the refresh (only relevant for `DB_TYPE=local`)token                                                        |
| `SESSION_SECRET`         | The secret key with which to sign the cookie store session                                                                                    |
| `DB_TYPE`                | The type of database to use (choose `local` for local Postgres db using Bun or `remote` for remote Postgres db using Bun and Supabase Client) |
| `DB_HOST`                | The host of the Postgres db                                                                                                                   |
| `DB_USER`                | The user of the Postgres db                                                                                                                   |
| `DB_PASSWORD`            | The password of the Postgres db                                                                                                               |
| `DB_NAME`                | The name of the Postgres db                                                                                                                   |
| `SUPABASE_URL`           | The Supabase URL (required for the client configuration), when `DB_TYPE=remote`                                                               |
| `SUPABASE_SECRET`        | The Supabase secret (required for the client configuration), when `DB_TYPE=remote`                                                            |
| `AUTH_CALLBACK_URL`      | The callback URL for remote login, when `DB_TYPE=remote`                                                                                      |

### Required Database

Wits requires a Postgres database to run. The connection details are configurable via environment variables (see above). For local deployment and testing, kubernetes resources for the database and application are provided in the `k8s` folder. In the [Makefile](Makefile) you can spin those up with `$ make k8s-up`. It can be brought down with `$ make k8s-down` . **Note**: Currently, bringing the database down also deletes all data. This behaviour is subject to change.

These commands requires [kubectl](https://kubernetes.io/docs/tasks/tools/install-kubectl/) to be installed on your machine.

**Important**: Make sure to create the secret for the database credentials, like so:

```shell
kubectl create secret generic postgres-credentials --from-literal=user=<your-user> --from-literal=password=<your-password> --from-literal=dbname=<your-db-name>
```

With a running database, the built application binary can be started by:

```shell
$ ./bin/wits
2024/03/07 00:27:08 INFO 💬 🖥️  (cmd/server.go) 🥦 Welcome to Wits!
2024/03/07 00:27:08 INFO 💬 💾 (pkg/storage/bun.go) InitBunWithPostgres()
2024/03/07 00:27:08 INFO 💬 💾 (pkg/storage/bun.go) CreatePostgresDB()
2024/03/07 00:27:08 INFO ✅ 💾 (pkg/storage/bun.go) CreatePostgresDB() -> 📂 Successfully created Postgresql db connection with host=127.0.0.1:5432
2024/03/07 00:27:08 INFO ✅ 💾 (pkg/storage/bun.go) InitBunWithPostgres() -> 📂 Successfully initialized Bun with Postgres db
2024/03/07 00:27:08 INFO 💬 🖥️  (cmd/server.go) configureLogging()
2024/03/07 00:27:08 INFO ✅ 🖥️  (cmd/server.go) configureLogging() -> 🗒️  OK with logLevel=INFO logFilePath=log/wits.log accessLogPath=log/access.log
2024/03/07 00:27:08 INFO 🚀 🖥️  (cmd/server.go) 🛜 Wits server is running at addr=127.0.0.1:3000

   ____    __
  / __/___/ /  ___
 / _// __/ _ \/ _ \
/___/\__/_//_/\___/ v4.11.4
High performance, minimalist Go web framework
https://echo.labstack.com
____________________________________O/_______
                                    O\
⇨ http server started on 127.0.0.1:3000
```

The built binary is explicitly ignored from source control (see [.gitignore](.gitignore)).

## Running the Application in a Kubernetes cluster

Wits provides the required resources to be deployed to a k8s cluster. If you are running a local cluster, e.g. through `minikube` you will want to add your Personal Access Token from your GitHub Account to be able to read your packages from the ghcr registry. You can do so by running:

```bash
$ kubectl create secret generic ghcr-secret --from-file=.dockerconfigjson=$HOME/.config/containers/auth.json --type=kubernetes.io/dockerconfigjson

secret/ghcr-secret created
```

## Running Tests

- Run the testsuite with coverage enabled:

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

- Generate the coverage results as html:

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

- Open the results in the browser:

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
