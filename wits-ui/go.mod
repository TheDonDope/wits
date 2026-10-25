// The browser client, not Go. Its own go.mod keeps it out of the module:
// an npm package (flatted) ships Go sources, and without this `go test ./...`
// built and covered them as part of wits.
module github.com/TheDonDope/wits/wits-ui

go 1.26
