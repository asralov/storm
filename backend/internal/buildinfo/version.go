// Package buildinfo provides the single source of the application version.
package buildinfo

import (
	_ "embed"
	"strings"
)

//go:embed VERSION
var version string

func Version() string { return strings.TrimSpace(version) }
