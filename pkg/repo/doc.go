// Package repo manages the .wits repository: creating it, finding it from a
// working directory, and reading the configuration and catalogs it holds.
//
// The layout deliberately mirrors a git repository. `wits init .` creates
// `.wits` the way `git init .` creates `.git`, and commands run from anywhere
// inside the tree by walking upwards until they find it.
package repo
