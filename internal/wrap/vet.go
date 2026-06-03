package wrap

// Vet executes `spin vet` in the current working directory.
//
// `go vet` is the only tool here — there is no preferred
// upgrade path (golangci-lint runs `go vet` internally as one of
// its checks, and exposing the user's lint preferences is a Phase
// 4 config-file feature, not a wrapper concern). vet.go exists so
// the user has a uniform `spin vet` command matching `spin run`,
// `spin build`, `spin test`, `spin fmt`.
func Vet() error {
	spec := ToolSpec{
		Name: "go",
		Args: []string{"vet", "./..."},
	}
	return runTool(spec.Name, spec.Args, nil)
}
