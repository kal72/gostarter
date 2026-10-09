package auth

import "errors"

var ErrUnavailable = errors.New("authentication unavailable")
var ErrCredentials = errors.New("invalid username or password")
var ErrSession = errors.New("failed creating session")
var ErrRequiredCredentials = errors.New("username and password are required")
