package backend

import (
	"os"
	"path/filepath"
	"runtime"
)

func getMachbaseNeoPath(defaultPath string) (string, error) {
	neoExePath := defaultPath
	if neoExePath == "" {
		selfPath, _ := os.Executable()
		selfDir := filepath.Dir(selfPath)
		if runtime.GOOS == "windows" {
			neoExePath = filepath.Join(selfDir, "machbase-neo.exe")
		} else {
			neoExePath = filepath.Join(selfDir, "machbase-neo")
		}
	}

	if _, err := os.Stat(neoExePath); err != nil {
		return "", err
	}

	return neoExePath, nil
}
