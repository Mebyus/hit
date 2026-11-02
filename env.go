package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type LineError struct {
	Text  string
	Label string
	Line  int // one based value
}

func (e *LineError) Error() string {
	return fmt.Sprintf("%s:%d: %s", e.Label, e.Line, e.Text)
}

// LoadEnv loads key=value pairs from specified file.
//
// Returns nil map if specified path is empty.
func LoadEnv(path string) (map[string]string, error) {
	if path == "" {
		return nil, nil
	}

	var g envloader
	g.init()
	err := g.load("", path)
	if err != nil {
		return nil, err
	}

	return g.env, nil
}

type envloader struct {
	// contains loaded key=value pairs
	env map[string]string

	// set of already visited files
	vis map[string]struct{}
}

func (g *envloader) init() {
	g.env = make(map[string]string)
	g.vis = make(map[string]struct{})
}

func (g *envloader) load(prefix string, path string) error {
	if strings.HasPrefix(path, "./") {
		path = filepath.Join(prefix, path)
	} else {
		path = filepath.Clean(path)
	}

	_, ok := g.vis[path]
	if ok {
		return nil
	}
	g.vis[path] = struct{}{}

	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	n := 0 // line number
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		n += 1
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			// skip empty lines and line comments
			continue
		}

		if strings.HasPrefix(line, "{") {
			// TODO: parse directive
			continue
		}

		reset := strings.HasPrefix(line, "+")
		if reset {
			line = strings.TrimSpace(line[1:])
		}

		name, value, ok := strings.Cut(line, "=")
		if !ok {
			return &LineError{
				Text:  "bad key=value format",
				Label: path,
				Line:  n,
			}
		}

		name = strings.TrimSpace(name)
		value = strings.TrimSpace(value)

		if name == "" {
			return &LineError{
				Text:  "empty name",
				Label: path,
				Line:  n,
			}
		}

		if !reset {
			_, ok = g.env[name]
			if ok {
				return &LineError{
					Text:  fmt.Sprintf("key \"%s\" already has value", name),
					Label: path,
					Line:  n,
				}
			}
		}

		g.env[name] = value
	}
	err = scanner.Err()
	if err != nil {
		return err
	}

	return nil
}
