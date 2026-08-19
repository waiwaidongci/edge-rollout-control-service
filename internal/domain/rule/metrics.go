package rule

import "math"

type Metrics struct {
	Total     int
	Succeeded int
	Failed    int
	TimedOut  int
	Offline   int
}

func (m Metrics) FailureRate() float64 {
	if m.Total <= 0 {
		return 0
	}
	return float64(m.Failed) / float64(m.Total)
}

func (m Metrics) TimeoutRate() float64 {
	if m.Total <= 0 {
		return 0
	}
	return float64(m.TimedOut) / float64(m.Total)
}

func (m Metrics) OfflineRate() float64 {
	if m.Total <= 0 {
		return 0
	}
	return float64(m.Offline) / float64(m.Total)
}

func (m Metrics) Valid() bool {
	return m.Total >= 0 && m.Succeeded >= 0 && m.Failed >= 0 && m.TimedOut >= 0 && m.Offline >= 0 && m.Succeeded+m.Failed+m.TimedOut <= m.Total
}

func (m Metrics) RoundedFailureRate() float64 {
	return math.Round(m.FailureRate()*10000) / 10000
}

func (r Rule) Decision(m Metrics) (bool, string) {
	if !m.Valid() || !r.Enabled || m.Total == 0 {
		return false, ""
	}
	if r.MaxFailureRate > 0 && m.FailureRate() >= r.MaxFailureRate {
		return true, "failure_rate"
	}
	if r.MaxOfflineRate > 0 && m.OfflineRate() >= r.MaxOfflineRate {
		return true, "offline_rate"
	}
	if r.MaxTimeoutRate > 0 && m.TimeoutRate() >= r.MaxTimeoutRate {
		return true, "timeout_rate"
	}
	return false, ""
}
