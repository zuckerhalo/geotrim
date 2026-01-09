package lib

import (
	"os"
	"path/filepath"
)

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func getFileSize(path string) (float64, error) {
	info, err := os.Stat(path)
	if err != nil {
		return 0, err
	}
	return float64(info.Size()) / 1024 / 1024, nil
}

func findGeodataFiles(dir string) (geoipPath, geositePath string) {
	files, err := os.ReadDir(dir)
	if err != nil {
		return "", ""
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}
		name := file.Name()
		if name == "geoip.dat" {
			geoipPath = filepath.Join(dir, name)
		}
		if name == "geosite.dat" {
			geositePath = filepath.Join(dir, name)
		}
	}

	return geoipPath, geositePath
}
