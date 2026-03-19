package cli

import (
	"fmt"
	"strings"

	"github.com/certyn/certyn-cli/internal/output"
)

func printHumanHeader(st output.Styler, kind string, title string) {
	fmt.Printf("%s %s\n", st.Badge(kind), st.Bold(strings.TrimSpace(title)))
}

func printHumanField(st output.Styler, label string, value string) {
	if strings.TrimSpace(value) == "" {
		return
	}
	fmt.Printf("  %s %s\n", st.Dim(fmt.Sprintf("%-14s", label)), value)
}

func printHumanItem(st output.Styler, summary string) {
	if strings.TrimSpace(summary) == "" {
		return
	}
	fmt.Printf("  - %s\n", summary)
}

func humanBool(st output.Styler, value bool) string {
	if value {
		return st.Green("yes")
	}
	return st.Dim("no")
}

func humanKVSummary(parts ...string) string {
	filtered := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			filtered = append(filtered, part)
		}
	}
	return strings.Join(filtered, ", ")
}
