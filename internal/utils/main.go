package utils

import (
	"io/ioutil"
	"os"
	"path"
)

func GetExecutePath() (string, error) {
	filePath, err := os.Executable()
	if err != nil {
		return "", err
	}
	return path.Dir(filePath), nil
}

func IsDirExists(path string) bool {
	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return false
	}
	return info.IsDir()
}

func IsFileExists(filename string) bool {
	found := false
	if filename != "" {
		info, err := os.Stat(filename)
		if err != nil {
			return false
		}
		found = !info.IsDir()
	}
	return found
}

func CreateReadStream(filename string) ([]byte, error) {
	file, err := ioutil.ReadFile(filename)

	return file, err
}
