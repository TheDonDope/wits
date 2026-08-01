// Package version holds the build information of the running binary. The
// values are injected through ldflags at build time and copied here by the
// main package, so any other package can report them without importing main.
package version

var (
	// Version is the version of the build, from the most recent git tag.
	Version = ""

	// CommitSHA is the commit the build was made from.
	CommitSHA = ""

	// CommitDate is the date of that commit.
	CommitDate = ""
)
