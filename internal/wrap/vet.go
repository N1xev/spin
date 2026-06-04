package wrap

// Vet wraps `go vet ./...` for the `spin vet` subcommand.
func Vet() error {
	spec := ToolSpec{
		Name: "go",
		Args: []string{"vet", "./..."},
	}
	return runTool(spec.Name, spec.Args, nil)
}
