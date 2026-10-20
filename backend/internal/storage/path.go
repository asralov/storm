package storage

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
)

func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return dataPath(runtime.GOOS, home, os.Getenv("LOCALAPPDATA"), os.Getenv("XDG_DATA_HOME"))
}
func dataPath(platform, home, local, xdg string) (string, error) {
	var directory string
	switch platform {
	case "windows":
		if local == "" {
			return "", errors.New("LOCALAPPDATA is required for Storm history")
		}
		directory = filepath.Join(local, "Storm")
	case "darwin":
		directory = filepath.Join(home, "Library", "Application Support", "Storm")
	default:
		if xdg != "" && filepath.IsAbs(xdg) {
			directory = filepath.Join(xdg, "storm")
		} else {
			directory = filepath.Join(home, ".local", "share", "storm")
		}
	}
	return filepath.Join(directory, "history.sqlite3"), nil
}
