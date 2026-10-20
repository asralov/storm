//go:build production

package webui

import (
	"io/fs"
	"strings"
	"testing"
)

func TestProductionBundle(t *testing.T) {
	files, err := Assets()
	if err != nil {
		t.Fatal(err)
	}
	index, err := fs.ReadFile(files, "index.html")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(index), "/assets/") {
		t.Fatal("index has no compiled assets")
	}
	matches, err := fs.Glob(files, "assets/*.js")
	if err != nil || len(matches) == 0 {
		t.Fatal("compiled JavaScript missing", err)
	}
}
