package version

import "runtime/debug"

var (
	// Version is the version of the build, from the most recent git tag. It is
	// the target the Makefile's ldflags write to.
	Version = ""

	// CommitSHA is the commit the build was made from.
	CommitSHA = ""

	// CommitDate is the date of that commit.
	CommitDate = ""
)

// String renders the version for display: the tag, with the abbreviated commit
// appended when there is one.
//
// Built with `go install` rather than the Makefile the ldflags are unset, so
// it falls back to the module version the Go toolchain stamps in, and finally
// to a plain admission that it was built from source.
func String() string {
	v := Version
	if v == "" {
		if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
			v = info.Main.Version
		} else {
			v = "unknown (built from source)"
		}
	}
	if len(CommitSHA) >= 7 {
		v += " (" + CommitSHA[:7] + ")"
	}
	return v
}
