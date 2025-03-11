<a name="unreleased"></a>
## [Unreleased]


<a name="v0.1.0"></a>
## v0.1.0 - 2025-03-11
### Chore
- update dependabot configuration
- add tailwindcss dev dependency
- initialize tailwindcss
- add Taskfile
- add create table sql script for user table
- air triggers templ generate before building go binary
- air also rebuilds tailwindcss
- remove codeql action
- clean up gitignore
- provide example .env file
- add tasks for database container management
- update daisyui to v4.7.3
- update dependencies
- clean up build files and remove unused dependencies
- update github build with dependency caching
- add Dockerfile
- update dependabot configuration
- **deps:** bump golang.org/x/net from 0.22.0 to 0.23.0
- **deps:** bump github.com/uptrace/bun/dialect/pgdialect
- **deps:** add [@tailwindcss](https://github.com/tailwindcss)/cli
- **deps:** bump docker/metadata-action
- **deps:** bump github.com/nedpals/supabase-go from 0.4.0 to 0.5.0
- **deps:** bump golang from 1.22-alpine to 1.24-alpine
- **deps:** bump github.com/labstack/echo/v4 from 4.12.0 to 4.13.3
- **deps:** bump docker/build-push-action
- **deps:** bump docker/login-action
- **deps:** bump codecov/codecov-action from 4 to 5
- **deps:** bump actions/attest-build-provenance from 1 to 2
- **deps:** bump github.com/uptrace/bun/extra/bundebug
- **deps:** bump golang.org/x/crypto from 0.35.0 to 0.36.0
- **deps:** bump github.com/golang-migrate/migrate/v4
- **deps:** bump github.com/a-h/templ from 0.2.747 to 0.3.833
- **deps:** bump golang.org/x/net from 0.24.0 to 0.33.0
- **deps:** bump golang.org/x/crypto from 0.25.0 to 0.33.0
- **deps:** bump github.com/uptrace/bun/dialect/pgdialect
- **deps:** bump github.com/docker/docker
- **deps:** bump github.com/a-h/templ from 0.2.731 to 0.2.747
- **deps:** bump golang.org/x/crypto from 0.24.0 to 0.25.0
- **deps:** bump github.com/a-h/templ from 0.2.707 to 0.2.731
- **deps:** bump github.com/gorilla/sessions from 1.2.2 to 1.3.0
- **deps:** bump golang.org/x/crypto from 0.23.0 to 0.24.0
- **deps:** bump github.com/uptrace/bun/extra/bundebug
- **deps:** bump github.com/uptrace/bun/dialect/pgdialect
- **deps:** bump github.com/uptrace/bun/dialect/pgdialect
- **deps:** bump github.com/labstack/echo/v4 from 4.11.4 to 4.12.0
- **deps:** bump github.com/a-h/templ from 0.2.663 to 0.2.707
- **deps:** bump golang.org/x/crypto from 0.21.0 to 0.23.0
- **deps:** bump github.com/golang-migrate/migrate/v4
- **deps:** add git-chglog integration
- **deps:** bump github.com/uptrace/bun/extra/bundebug
- **deps:** bump github.com/a-h/templ from 0.2.598 to 0.2.663
- **deps:** bump github.com/labstack/echo-jwt/v4 from 4.2.0 to 4.3.0
- **deps:** bump github.com/docker/docker
- **deps:** bump github.com/uptrace/bun/extra/bundebug
- **deps:** bump docker/build-push-action from 6.13.0 to 6.14.0
- **deps:** bump golang.org/x/crypto from 0.33.0 to 0.35.0
- **deps:** bump github.com/uptrace/bun from 1.2.10 to 1.2.11
- **deps:** bump docker/metadata-action
- **deps:** bump docker/build-push-action from 6.14.0 to 6.15.0
- **deps-dev:** bump daisyui from 5.0.0-beta.8 to 5.0.0
- **deps-dev:** bump tailwindcss from 4.0.6 to 4.0.8
- **deps-dev:** bump tailwindcss from 4.0.0 to 4.0.6
- **deps-dev:** bump tailwindcss from 3.4.4 to 4.0.6
- **deps-dev:** bump tailwindcss from 4.0.8 to 4.0.12
- **deps-dev:** bump daisyui from 4.12.2 to 4.12.23

### Docs
- update dependabot documentation linkout
- update readme
- clean up readme
- update build instructions
- re-go-ify docs
- update godocs

### Feat
- add local logout
- login with supabase
- add github workflow to publish image to ghcr
- initial commit
- provide full k8s resources
- load account data for user
- save user id to encoded session
- convert to k8s
- integrate account table
- remove sqlite dependencies and unify postgresql support
- add postgresql database and migrations
- integrate gorilla sessions jwt retrival for local login and register
- use gorilla/sessions for supabase login and logout
- add gorilla/sessions cookie store to auth handler
- create log directory if not existing
- add auth package
- add creation methods for handlers based on db type
- improve logging and error handling
- configure application logging
- add redirect param to login
- provide auth middleware to inject authorized user information
- provide settings page and properly clear cookies on logout
- provide login with google
- extract middleware function to provide user in context
- beautify the logs
- provide access to authenticated user in views and handlers
- properly login remotely and configure jwt signing appropriately
- contextualize logging messages
- htmx-ify login, register, and logout
- do not commit 3rd party js and css assets
- use make in github ci build instead of task
- run npx [@tailwindcss](https://github.com/tailwindcss)/upgrade
- implement user logout
- enable separate storage implementations
- register with supabase
- add logout handler and wire up with navigation
- integrate with supabase
- add top margin to submit button on register form
- remove error placeholders
- beautify the dashboard
- beautify the register page
- beautify the login page
- add initial navigation bar
- niceify info and error logging
- provide Makefile and move assets to public directory
- add register route
- add some styling to dashboard
- integrate gorm for database connection
- wire up sqlite3 database
- provide in-memory test users
- redirect index route depending on logged in state of user
- add login with jwt and cookies
- add get route for login
- setup echo server
- compile tailwindcss output
- **cli:** add strain add command
- **cli:** add bubbletea dependencies and first main menu version
- **pkg:** add strain list model
- **pkg:** add new tui package
- **pkg:** add new cannabis package
- **pkg:** add types for strains

### Fix
- configure daisyui by upgrading to v5 beta
- remove daisyui plugin during v4 migration
- update audited dependencies
- make build runnable again
- use fully qualified image name for postgres
- make github successful again
- clean up logging
- clean up cookies
- hopefully make go build runnable again
- make coverage build on github runnable again
- Make github builds runnable again
- remove generated go files from git

### Refac
- **cmd:** clean up cli main.go
- **pkg:** extract interfaces for service and storage
- **pkg:** move strain constructor from form to tui renderer

### Refactor
- remove sqlite initializer
- remove unused code
- group auth handlers by category into separate files
- clean up logging
- move jwt logic to auth package
- remove env variable helpers
- extract user repository
- extract route configuration to separate function
- clean up jwt signing and cookie code
- extract login and register handler
- extract login form to own function
- rename dsn to sqlite dsn
- extract http error handling to middleware
- merge login and register to auth
- clean up some code
- extract json middleware to handlers
- rename templates.go to handlers.go
- rename input.css to app.css
- prepare to extract navigation and add fontawesome and jquery
- rename container to app layout
- extract home handler into own file and add generic http error handler
- move server and database address to env variables
- extract environment variables to .env file
- clean up server

### Revert
- roll back to tailwindcss v3 to use upgrade tool

### Test
- clean up test
- make user repo test runnable
- add user_repo tests (wip)


[Unreleased]: https://github.com/TheDonDope/wits/compare/v0.1.0...HEAD
