// Command spin is a language-agnostic scaffolder for external
// templates. `spin new <name> [<template>]` scaffolds a project
// from any git repo, local path, or pinned template that has a
// spin.toml + _base/ tree. No built-in templates.
package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"charm.land/fang/v2"

	"github.com/N1xev/spin/cmd"
	"github.com/N1xev/spin/internal/version"
)

func main() {
	err := fang.Execute(
		context.Background(),
		cmd.RootCmd(),
		fang.WithVersion(version.Version),
	)
	if err == nil {
		return
	}
	if errors.Is(err, cmd.ErrCancelled) {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(130)
	}
	os.Exit(1)
}
