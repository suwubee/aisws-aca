package setupwizard

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func parseDotEnvFile(path string) (map[string]string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, errors.New("env path is required")
	}

	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	env := make(map[string]string)
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		if key == "" {
			continue
		}
		value = strings.TrimSpace(value)
		value = strings.Trim(value, `"'`)
		env[key] = value
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return env, nil
}

type dotenvWriteOptions struct {
	headerComment string
	mode          os.FileMode
}

func writeDotEnvFile(path string, env map[string]string, opts dotenvWriteOptions) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("env path is required")
	}
	if env == nil {
		return errors.New("env is nil")
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	mode := opts.mode
	if mode == 0 {
		mode = 0600
	}

	var buf bytes.Buffer
	header := strings.TrimSpace(opts.headerComment)
	if header != "" {
		for _, line := range strings.Split(header, "\n") {
			line = strings.TrimRight(line, "\r")
			if line == "" {
				buf.WriteString("\n")
				continue
			}
			if !strings.HasPrefix(strings.TrimSpace(line), "#") {
				buf.WriteString("# ")
				buf.WriteString(line)
			} else {
				buf.WriteString(line)
			}
			buf.WriteString("\n")
		}
		buf.WriteString("\n")
	}

	keys := make([]string, 0, len(env))
	for k := range env {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	for _, key := range keys {
		value := env[key]
		buf.WriteString(fmt.Sprintf("%s=%s\n", key, escapeDotEnvValue(value)))
	}

	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, buf.Bytes(), mode); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func escapeDotEnvValue(value string) string {
	v := strings.TrimSpace(value)
	if v == "" {
		return ""
	}

	needsQuote := strings.ContainsAny(v, " #\t\r\n\"'")
	if !needsQuote {
		return v
	}

	// Use double quotes and escape internal quotes/backslashes.
	replacer := strings.NewReplacer(`\`, `\\`, `"`, `\"`)
	return `"` + replacer.Replace(v) + `"`
}

