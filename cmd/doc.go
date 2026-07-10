// Package cmd wires the spin cobra command tree. Subcommand files
// (new.go, list.go, ...) attach themselves to rootCmd via init().
// RootCmd() returns the fully-populated tree for main and tests.
package cmd
