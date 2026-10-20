//go:build !production

package webui

import (
	"errors"
	"io/fs"
)

func Assets() (fs.FS, error) {
	return nil, errors.New("dashboard is not embedded; use the release build script or run cmd/storm-server with Vite")
}
