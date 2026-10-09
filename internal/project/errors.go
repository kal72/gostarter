package project

import "errors"

var ErrUnavailable = errors.New("application service unavailable")

type Failure struct {
	Status        int
	Code, Message string
}

func (e *Failure) Error() string { return e.Message }
