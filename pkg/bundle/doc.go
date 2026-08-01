// Package bundle reads and writes a whole repository as a single compact file.
//
// A bundle is to a Wits repository what `git bundle` is to a git repository: a
// portable, self-contained copy of the history that can be carried to another
// machine and unpacked there. It holds the catalogs and every event, and it
// round-trips exactly — restoring a bundle produces a journal whose hashes
// match the one it was written from.
//
// The format is deliberately plain text. The journal is a medical record that
// may outlive this program, so an archive of it should be legible with nothing
// but a text editor. Being line-oriented, it also diffs well in git: appending
// an event appends a line.
//
// It is nonetheless small. Four years of real history — 1986 events across 50
// products — is 822 KB as the raw journal, 141 KB as xz-compressed JSON, and
// roughly 17 KB as a bundle, because most of what the journal stores is
// derivable: the sequence numbers, the account pairs, and the whole hash chain
// are all recomputed on restore rather than written down.
package bundle
