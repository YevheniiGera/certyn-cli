package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/certyn/certyn-cli/internal/cli"
)

var version = "dev"

func main() {
	cli.Version = version
	root := cli.NewRootCommand()
	if err := root.Execute(); err != nil {
		if ce := findCommandError(err); ce != nil {
			if ce.Message != "" {
				fmt.Fprintln(os.Stderr, ce.Message)
			}
			if ce.Err != nil {
				fmt.Fprintln(os.Stderr, ce.Err)
			}
			os.Exit(ce.Code)
		}

		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func findCommandError(err error) *cli.CommandError {
	if err == nil {
		return nil
	}
	var cmdErr *cli.CommandError
	if errors.As(err, &cmdErr) {
		return cmdErr
	}
	return nil
}
