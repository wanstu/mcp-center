package server

import (
	"context"
	"sort"
	"time"

	mcpcenter "github.com/wanstu/mcp-center-core"
)

const defaultScheduledProbeInterval = 60 * time.Second

type scheduledProbeState struct {
	Running     bool
	Interval    time.Duration
	NextProbeAt time.Time
	LastChecked time.Time
	LastState   string
	LastError   string
}

type scheduledProbeView struct {
	MCPID               string     `json:"mcp_id"`
	Enabled             bool       `json:"enabled"`
	Active              bool       `json:"active"`
	Running             bool       `json:"running"`
	IntervalSeconds     int64      `json:"interval_seconds"`
	ProbeTimeoutSeconds int64      `json:"probe_timeout_seconds"`
	NextProbeAt         *time.Time `json:"next_probe_at,omitempty"`
	LastCheckedAt       *time.Time `json:"last_checked_at,omitempty"`
	LastState           string     `json:"last_state,omitempty"`
	LastError           string     `json:"last_error,omitempty"`
}

func scheduledProbeInterval(policy mcpcenter.HealthPolicy) time.Duration {
	if policy.CheckIntervalSeconds <= 0 {
		return defaultScheduledProbeInterval
	}
	return time.Duration(policy.CheckIntervalSeconds) * time.Second
}

func (a *App) runScheduledProbeLoop(ctx context.Context) {
	a.runScheduledProbePass(ctx, time.Now().UTC())
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			a.runScheduledProbePass(ctx, now.UTC())
		}
	}
}

func (a *App) runScheduledProbePass(ctx context.Context, now time.Time) {
	definitions, err := a.core.List()
	if err != nil {
		return
	}

	active := make(map[string]struct{}, len(definitions))
	for _, def := range definitions {
		if def.Disabled || !def.HealthPolicy.HealthCheckEnabled {
			continue
		}
		active[def.ID] = struct{}{}
		interval := scheduledProbeInterval(def.HealthPolicy)

		a.scheduledProbeMu.Lock()
		state := a.scheduledProbes[def.ID]
		if state == nil {
			state = &scheduledProbeState{Interval: interval, NextProbeAt: now}
			a.scheduledProbes[def.ID] = state
		} else if state.Interval != interval {
			state.Interval = interval
			state.NextProbeAt = now
		}
		if state.NextProbeAt.IsZero() {
			state.NextProbeAt = now
		}
		if state.Running || now.Before(state.NextProbeAt) {
			a.scheduledProbeMu.Unlock()
			continue
		}
		state.Running = true
		state.NextProbeAt = now.Add(interval)
		a.scheduledProbeMu.Unlock()

		go a.runScheduledProbe(ctx, def.ID)
	}

	a.scheduledProbeMu.Lock()
	for id, state := range a.scheduledProbes {
		if _, ok := active[id]; !ok && !state.Running {
			delete(a.scheduledProbes, id)
		}
	}
	a.scheduledProbeMu.Unlock()
}

func (a *App) runScheduledProbe(ctx context.Context, id string) {
	status, err := a.core.Probe(ctx, id)
	if ctx.Err() != nil {
		a.scheduledProbeMu.Lock()
		if state := a.scheduledProbes[id]; state != nil {
			state.Running = false
		}
		a.scheduledProbeMu.Unlock()
		return
	}
	a.recordScheduledProbeResult(id, status, err, false)
}

func (a *App) recordScheduledProbeResult(id string, status mcpcenter.HealthStatus, err error, resetNext bool) {
	now := time.Now().UTC()
	a.scheduledProbeMu.Lock()
	defer a.scheduledProbeMu.Unlock()

	state := a.scheduledProbes[id]
	if state == nil {
		def, getErr := a.core.Get(id)
		if getErr != nil || !def.HealthPolicy.HealthCheckEnabled {
			return
		}
		state = &scheduledProbeState{Interval: scheduledProbeInterval(def.HealthPolicy)}
		a.scheduledProbes[id] = state
	}
	state.Running = false
	state.LastChecked = now
	if !status.CheckedAt.IsZero() {
		state.LastChecked = status.CheckedAt.UTC()
	}
	if err != nil {
		state.LastState = "error"
		state.LastError = err.Error()
	} else {
		state.LastState = string(status.State)
		state.LastError = status.Message
	}
	if resetNext {
		if state.Interval <= 0 {
			state.Interval = defaultScheduledProbeInterval
		}
		state.NextProbeAt = now.Add(state.Interval)
	}
}

func (a *App) resetScheduledProbe(id string) {
	a.scheduledProbeMu.Lock()
	delete(a.scheduledProbes, id)
	a.scheduledProbeMu.Unlock()
}

func (a *App) scheduledProbeViews() []scheduledProbeView {
	definitions, err := a.core.List()
	if err != nil {
		return nil
	}

	a.scheduledProbeMu.RLock()
	defer a.scheduledProbeMu.RUnlock()

	out := make([]scheduledProbeView, 0)
	for _, def := range definitions {
		if !def.HealthPolicy.HealthCheckEnabled {
			continue
		}
		interval := scheduledProbeInterval(def.HealthPolicy)
		view := scheduledProbeView{
			MCPID:               def.ID,
			Enabled:             true,
			Active:              !def.Disabled,
			IntervalSeconds:     int64(interval / time.Second),
			ProbeTimeoutSeconds: def.HealthPolicy.ProbeTimeoutSeconds,
		}
		if state := a.scheduledProbes[def.ID]; state != nil {
			view.Running = state.Running
			if !state.NextProbeAt.IsZero() && !def.Disabled {
				value := state.NextProbeAt.UTC()
				view.NextProbeAt = &value
			}
			if !state.LastChecked.IsZero() {
				value := state.LastChecked.UTC()
				view.LastCheckedAt = &value
			}
			view.LastState = state.LastState
			view.LastError = state.LastError
		}
		out = append(out, view)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].MCPID < out[j].MCPID })
	return out
}
