# Wits - The Weed Information Tracking System

[![codecov](https://codecov.io/gh/TheDonDope/wits/graph/badge.svg?token=lMa764i83e)](https://codecov.io/gh/TheDonDope/wits) [![CodeQL](https://github.com/TheDonDope/wits/actions/workflows/codeql.yml/badge.svg)](https://github.com/TheDonDope/wits/actions/workflows/codeql.yml/)

Wits aims to help cannabis patients and users to manage and monitor their cannabis consumption and inventory.

## Building and Running

To build the binary, run the following (alternatively `$ task build` if you are using [Task](https://taskfile.dev/#/)):

```shell
$ go build -v -o ./wits ./cmd/server.go
[Empty output on success]
```

Afterwards the application can be started by:

```shell
$ ./wits
Welcome to Wits!

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
$ go test -race -v ./... -coverprofile coverage.out
?    github.com/TheDonDope/wits/pkg/types [no test files]
?    github.com/TheDonDope/wits/pkg/view [no test files]
 github.com/TheDonDope/wits/cmd  coverage: 0.0% of statements
 github.com/TheDonDope/wits/pkg/handler  coverage: 0.0% of statements
 github.com/TheDonDope/wits/pkg/auth  coverage: 0.0% of statements
 github.com/TheDonDope/wits/pkg/view/dashboard  coverage: 0.0% of statements
 github.com/TheDonDope/wits/pkg/view/login  coverage: 0.0% of statements
 github.com/TheDonDope/wits/pkg/view/layout  coverage: 0.0% of statements
```

- Generate the coverage results as html (alternatively `$ task cover` if you are using [Task](https://taskfile.dev/#/)):

```shell
$ go tool cover -html coverage.out -o coverage.html
[Empty output on success]
```

- Open the results in the browser (alternatively `$ task show-cover` if you are using [Task](https://taskfile.dev/#/)):

```shell
$ open coverage.html
<Opens Browser>
```

Both the `coverage.out` as well as the `coverage.html` are explicitly ignored from source control (see [.gitignore](.gitignore)).
