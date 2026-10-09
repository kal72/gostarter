package upstream

import "errors"

var ErrUnavailable = errors.New("upstream service unavailable")

var ErrSyncBusy = errors.New("sync is already queued or running")

var ErrValidationRunning = errors.New("cannot sync while validation is running")
