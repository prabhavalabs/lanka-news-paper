package config

import (
	"bufio"
	"os"
	"strings"
)

func LoadOptionalDotEnv(paths ...string) {
	for _, path := range paths {
		file, err := os.Open(path)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			key, value, ok := strings.Cut(line, "=")
			if !ok {
				continue
			}
			key = strings.TrimSpace(key)
			if os.Getenv(key) != "" {
				continue
			}
			_ = os.Setenv(key, strings.TrimSpace(value))
		}
		_ = file.Close()
	}
}
