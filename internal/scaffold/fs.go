// Package scaffold: template-FS abstraction so the walker transparently
// reads from go:embed or an external --template-repo clone.
package scaffold

import (
	"io/fs"
	"os"
)

// templateFS is the interface the template walker requires: fs.FS
// plus ReadDir and ReadFile. Both embed.FS and os.DirFS results satisfy it.
type templateFS interface {
	fs.FS
	fs.ReadDirFS
	fs.ReadFileFS
}

// currentFS returns the FS the walker should read from. The type
// assertion to templateFS is a compile-time guard: if os.DirFS's
// result ever stops satisfying one of the three interface methods,
// the failure surfaces here rather than as a nil-method call deep
// in fs.WalkDir.
func currentFS(externalDir string) templateFS {
	if externalDir == "" {
		return any(FS).(templateFS)
	}
	return os.DirFS(externalDir).(templateFS)
}
