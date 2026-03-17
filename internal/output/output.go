package output

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

type Printer struct {
	JSON bool
}

func (p Printer) Emit(value any, human string) error {
	if p.JSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(value)
	}
	fmt.Println(human)
	return nil
}

func (p Printer) EmitJSON(value any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(value)
}

func (p Printer) Printf(format string, args ...any) {
	if p.JSON {
		return
	}
	fmt.Printf(format, args...)
}

func WriteGitHubOutputs(values map[string]string) error {
	path := os.Getenv("GITHUB_OUTPUT")
	if path == "" {
		return nil
	}
	if len(values) == 0 {
		return nil
	}

	file, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o600)
	if err != nil {
		return err
	}
	defer file.Close()

	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, key := range keys {
		if _, err := fmt.Fprintf(file, "%s=%s\n", key, sanitize(values[key])); err != nil {
			return err
		}
	}
	return nil
}

func sanitize(value string) string {
	// Keep GITHUB_OUTPUT simple single-line values.
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.ReplaceAll(value, "\r", " ")
	return value
}
