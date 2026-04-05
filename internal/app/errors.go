package app

import "errors"

const (
	ExitSuccess  = 0
	ExitFindings = 1
	ExitInput    = 2
	ExitBaseline = 3
)

type exitCoder interface {
	ExitCode() int
}

type codedError struct {
	code int
	err  error
}

func (e codedError) Error() string {
	return e.err.Error()
}

func (e codedError) Unwrap() error {
	return e.err
}

func (e codedError) ExitCode() int {
	return e.code
}

func withExitCode(err error, code int) error {
	if err == nil {
		return nil
	}
	return codedError{code: code, err: err}
}

func ExitCode(err error) int {
	if err == nil {
		return ExitSuccess
	}

	var coder exitCoder
	if errors.As(err, &coder) {
		return coder.ExitCode()
	}
	return ExitFindings
}
