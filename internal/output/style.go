package output

import (
	"fmt"
	"os"
	"strings"
)

type Styler struct {
	Enabled bool
}

func NewStyler() Styler {
	return Styler{Enabled: shouldColorize()}
}

func (s Styler) Bold(text string) string {
	return s.wrap("1", text)
}

func (s Styler) Dim(text string) string {
	return s.wrap("2", text)
}

func (s Styler) Green(text string) string {
	return s.wrap("32", text)
}

func (s Styler) Yellow(text string) string {
	return s.wrap("33", text)
}

func (s Styler) Blue(text string) string {
	return s.wrap("34", text)
}

func (s Styler) Red(text string) string {
	return s.wrap("31", text)
}

func (s Styler) Cyan(text string) string {
	return s.wrap("36", text)
}

func (s Styler) Badge(kind string) string {
	label := strings.ToUpper(strings.TrimSpace(kind))
	if label == "" {
		label = "INFO"
	}

	switch strings.ToLower(strings.TrimSpace(kind)) {
	case "ok", "pass", "success":
		return s.Bold(s.Green(fmt.Sprintf("[%s]", label)))
	case "warn", "warning", "pending":
		return s.Bold(s.Yellow(fmt.Sprintf("[%s]", label)))
	case "fail", "error", "blocked":
		return s.Bold(s.Red(fmt.Sprintf("[%s]", label)))
	default:
		return s.Bold(s.Blue(fmt.Sprintf("[%s]", label)))
	}
}

func (s Styler) Status(value string) string {
	normalized := strings.ToLower(strings.TrimSpace(value))
	switch normalized {
	case "passed", "success", "completed", "ok":
		return s.Green(value)
	case "failed", "error", "aborted", "cancelled", "canceled", "blocked":
		return s.Red(value)
	case "running", "pending", "queued", "in_progress":
		return s.Yellow(value)
	default:
		return s.Cyan(value)
	}
}

func (s Styler) wrap(code, text string) string {
	if !s.Enabled || strings.TrimSpace(text) == "" {
		return text
	}
	return "\x1b[" + code + "m" + text + "\x1b[0m"
}

func shouldColorize() bool {
	if os.Getenv("NO_COLOR") != "" {
		return false
	}

	if force := strings.TrimSpace(os.Getenv("CLICOLOR_FORCE")); force != "" && force != "0" {
		return true
	}

	term := strings.TrimSpace(os.Getenv("TERM"))
	if term == "" || strings.EqualFold(term, "dumb") {
		return false
	}

	info, err := os.Stdout.Stat()
	if err != nil {
		return false
	}

	return (info.Mode() & os.ModeCharDevice) != 0
}
