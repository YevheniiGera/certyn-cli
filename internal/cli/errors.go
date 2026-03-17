package cli

const (
	ExitOK          = 0
	ExitGateFailed  = 1
	ExitUsage       = 2
	ExitAuth        = 3
	ExitNotFound    = 4
	ExitTimeout     = 5
	ExitInterrupted = 130
)

type CommandError struct {
	Code    int
	Message string
	Err     error
}

func (e *CommandError) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	if e.Err != nil {
		return e.Err.Error()
	}
	return "command failed"
}

func (e *CommandError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func usageError(msg string, err error) error {
	return &CommandError{Code: ExitUsage, Message: msg, Err: err}
}

func authError(msg string, err error) error {
	return &CommandError{Code: ExitAuth, Message: msg, Err: err}
}

func notFoundError(msg string, err error) error {
	return &CommandError{Code: ExitNotFound, Message: msg, Err: err}
}

func timeoutError(msg string, err error) error {
	return &CommandError{Code: ExitTimeout, Message: msg, Err: err}
}
