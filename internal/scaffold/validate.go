// Package scaffold: Validate enforces project name + directory constraints.
//
// The Plan 02 implementation in Task 2 will replace this stub with the real
// ModuleSegmentRegex + IsValidGoModuleSegment + existing-directory check.
// For Task 1 we ship a no-op so the cmd/new.go wiring compiles end-to-end
// and ResolveFlags can be tested in isolation.
package scaffold

// Validate returns nil if the Project's Name is well-formed and the
// target directory does not conflict with an existing path. Returns a
// descriptive error otherwise.
//
// Stub implementation for Task 1; real implementation lands in Task 2.
func (p *Project) Validate() error {
	return nil
}
