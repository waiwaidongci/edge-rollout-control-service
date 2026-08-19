package rule

import (
	"context"
	"errors"
	"time"
)

var ErrInvalid = errors.New("invalid rollout rule")

type Rule struct {
	ID, Name                                       string
	MaxFailureRate, MaxOfflineRate, MaxTimeoutRate float64
	CriticalLabels                                 map[string]string
	Enabled                                        bool
	CreatedAt, UpdatedAt                           time.Time
}
type Filter struct {
	Enabled       *bool
	Query         string
	Limit, Offset int
}

func (r Rule) Validate() error {
	if r.ID == "" || r.Name == "" || r.MaxFailureRate < 0 || r.MaxFailureRate > 1 || r.MaxOfflineRate < 0 || r.MaxOfflineRate > 1 || r.MaxTimeoutRate < 0 || r.MaxTimeoutRate > 1 {
		return ErrInvalid
	}
	return nil
}
func (r Rule) Evaluate(fail, offline, timeout, total int) (bool, string, error) {
	if total < 0 || fail < 0 || offline < 0 || timeout < 0 || fail > total || offline > total || timeout > total {
		return false, "", ErrInvalid
	}
	if !r.Enabled || total == 0 {
		return false, "", nil
	}
	if r.MaxFailureRate > 0 && float64(fail)/float64(total) >= r.MaxFailureRate {
		return true, "failure_rate", nil
	}
	if r.MaxOfflineRate > 0 && float64(offline)/float64(total) >= r.MaxOfflineRate {
		return true, "offline_rate", nil
	}
	if r.MaxTimeoutRate > 0 && float64(timeout)/float64(total) >= r.MaxTimeoutRate {
		return true, "timeout_rate", nil
	}
	return false, "", nil
}

type Repository interface {
	CreateRule(context.Context, Rule) error
	UpdateRule(context.Context, Rule) error
	GetRule(context.Context, string) (Rule, error)
	ListRules(context.Context, Filter) ([]Rule, int, error)
}
