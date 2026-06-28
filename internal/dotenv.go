package internal

import (
	"bufio"
	"log"
	"os"
	"strings"
)

func LoadDotenv(filenames ...string) {
	if len(filenames) == 0 {
		filenames = []string{".env"}
	}
	for _, filename := range filenames {
		f, err := os.Open(filename)
		if err != nil {
			continue
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			line := strings.TrimSpace(scanner.Text())
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			parts := strings.SplitN(line, "=", 2)
			if len(parts) != 2 {
				continue
			}
			key := strings.TrimSpace(parts[0])
			val := strings.TrimSpace(parts[1])
			val = strings.Trim(val, `"'`)
			if key == "" {
				continue
			}
			if os.Getenv(key) != "" {
				continue
			}
			os.Setenv(key, val)
		}
		if err := scanner.Err(); err != nil {
			log.Printf("LoadDotenv: error reading %s: %v", filename, err)
		}
	}
}
