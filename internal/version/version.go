package version

import (
	"regexp"
	"runtime/debug"
	"strings"
)

const defaultVersion = "0.1.0"

var Version = defaultVersion
var pseudoVersion = regexp.MustCompile(`\.[0-9]{14}-`)

func init() {
	if Version != defaultVersion {
		return // -ldflags already set an explicit version
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}
	v := info.Main.Version
	if v == "" || v == "(devel)" || strings.HasSuffix(v, "+dirty") ||
		pseudoVersion.MatchString(v) {
		return
	}
	Version = v
}
