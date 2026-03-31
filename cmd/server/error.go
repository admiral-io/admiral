package server

type exitError struct {
	err     error
	code    int
	details string
}

func wrapErrorWithCode(err error, code int, details string) *exitError {
	return &exitError{
		err:     err,
		code:    code,
		details: details,
	}
}

func (e *exitError) Error() string {
	return e.err.Error()
}
