// The browser client, not Go. Its own go.mod keeps it out of the module:
// an npm package (flatted) ships Go sources, and without this `go test ./...`
// built and covered them as part of wits. Nothing is built here, so it names
// no Go version — one it named, scanners read as a standard library in use.
module github.com/TheDonDope/wits/wits-ui
