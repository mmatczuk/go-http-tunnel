package fileutil

import (
	"bufio"
	"os"
	"strings"
)

type AppConfigProperties map[string]string

func ReadPropertiesFile(filename string) (AppConfigProperties, error) {
	config := AppConfigProperties{}

	if len(filename) == 0 {
		return config, nil
	}
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if equal := strings.Index(line, ";"); equal >= 0 {
			if key := strings.TrimSpace(line[:equal]); len(key) > 0 {
				value := ""
				if len(line) > equal {
					value = strings.TrimSpace(line[equal+1:])
				}
				config[key] = value
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return config, nil
}

// IsDirectory determines if a file represented by `path` is a directory or not
func IsDirectory(path string) (bool, error) {
	fileInfo, err := os.Stat(path)
	if err != nil {
		return false, err
	}
	return fileInfo.IsDir(), err
}
