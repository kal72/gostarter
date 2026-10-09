package project

import (
	"errors"
	"gostarter/internal/project/plan"
	"time"
)

var ErrPlanExpired = errors.New("plan expired")

type StoredPlan struct {
	ID                   string
	Plan                 plan.Plan
	Status               string
	ContentSHA256        string
	ValidationIdentity   string
	Diagnostic           string
	CreatedAt, ExpiresAt time.Time
}
