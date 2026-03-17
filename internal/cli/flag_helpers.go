package cli

import (
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
)

func parseOptionalBool(flagName, raw string) (*bool, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, nil
	}

	parsed, err := strconv.ParseBool(trimmed)
	if err != nil {
		return nil, usageError(fmt.Sprintf("invalid value for --%s: %q (expected true or false)", flagName, raw), err)
	}
	return &parsed, nil
}

func normalizeEnum(flagName, raw string, allowed []string) (string, error) {
	trimmed := strings.ToLower(strings.TrimSpace(raw))
	if trimmed == "" {
		return "", nil
	}
	if slices.Contains(allowed, trimmed) {
		return trimmed, nil
	}
	return "", usageError(
		fmt.Sprintf("invalid value for --%s: %q (allowed: %s)", flagName, raw, strings.Join(allowed, ", ")),
		nil,
	)
}

func normalizeCSVEnum(flagName, raw string, allowed []string) (string, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", nil
	}

	parts := strings.Split(trimmed, ",")
	normalized := make([]string, 0, len(parts))
	for _, part := range parts {
		value, err := normalizeEnum(flagName, part, allowed)
		if err != nil {
			return "", err
		}
		if value == "" {
			continue
		}
		normalized = append(normalized, value)
	}

	return strings.Join(normalized, ","), nil
}

func explicitFlagString(cmd *cobra.Command, name string) string {
	flag := cmd.Flags().Lookup(name)
	if flag == nil || !flag.Changed {
		return ""
	}
	return strings.TrimSpace(flag.Value.String())
}

func requireAnyFlagChanged(cmd *cobra.Command, names ...string) error {
	for _, name := range names {
		flag := cmd.Flags().Lookup(name)
		if flag != nil && flag.Changed {
			return nil
		}
	}
	return usageError("no update fields provided", nil)
}

func valueOrDash(value string) string {
	if strings.TrimSpace(value) == "" {
		return "-"
	}
	return value
}

func ptrStringOrDash(value *string) string {
	if value == nil {
		return "-"
	}
	return valueOrDash(*value)
}
