package cli

import (
	"encoding/base64"
	"fmt"
	"os"
	"strings"
)

func readFileAsBase64(path string) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "", usageError("missing file path", nil)
	}

	raw, err := os.ReadFile(trimmed)
	if err != nil {
		return "", usageError(fmt.Sprintf("failed to read file '%s'", trimmed), err)
	}
	if len(raw) == 0 {
		return "", usageError(fmt.Sprintf("file '%s' is empty", trimmed), nil)
	}

	return base64.StdEncoding.EncodeToString(raw), nil
}
