package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"ppeelink/throughputcore"
	"sync"
	"time"
)

type throughputAssignment struct {
	Jobs []throughputcore.Job `json:"jobs"`
}
type throughputRunning struct {
	cancel  context.CancelFunc
	done    chan struct{}
	mu      sync.Mutex
	pending *throughputcore.Report
}

func (r *throughputRunning) retry(ctx context.Context, client *http.Client, panel, token string) {
	r.mu.Lock()
	v := r.pending
	r.mu.Unlock()
	if v == nil {
		return
	}
	if postAgentThroughput(ctx, client, panel, token, "report", v) == nil {
		r.mu.Lock()
		if r.pending == v {
			r.pending = nil
		}
		r.mu.Unlock()
	}
}
func (r *throughputRunning) hasPending() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.pending != nil
}

// Poll responses are desired state: disappearance cancels an active process.
// Completed IDs remain acknowledged until removed, avoiding replay on slow polls.
func runAgentThroughputLoop(ctx context.Context, client *http.Client, panel, token string) {
	active := map[string]*throughputRunning{}
	finished := map[string]bool{}
	defer func() {
		for _, r := range active {
			r.cancel()
		}
		for _, r := range active {
			<-r.done
		}
	}()
	throughputcore.CleanupStaleAuthDirs(time.Now())
	cap := throughputcore.Inspect(ctx)
	_ = postAgentThroughput(ctx, client, panel, token, "capabilities", cap)
	lastCapability := time.Now()
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	for {
		jobs, err := fetchAgentThroughputJobs(ctx, client, panel, token)
		if err == nil {
			desired := map[string]throughputcore.Job{}
			for _, j := range jobs {
				desired[fmt.Sprintf("%d:%s", j.ID, j.Role)] = j
			}
			for key, r := range active {
				if _, ok := desired[key]; !ok {
					r.cancel()
					<-r.done
					delete(active, key)
					continue
				}
				r.retry(ctx, client, panel, token)
				select {
				case <-r.done:
					if !r.hasPending() {
						delete(active, key)
						finished[key] = true
					}
				default:
				}
			}
			for key := range finished {
				if _, ok := desired[key]; !ok {
					delete(finished, key)
				}
			}
			for key, j := range desired {
				if finished[key] || active[key] != nil || len(active) > 0 {
					continue
				}
				jobCtx, cancel := context.WithCancel(ctx)
				r := &throughputRunning{cancel: cancel, done: make(chan struct{})}
				active[key] = r
				go func(j throughputcore.Job, r *throughputRunning) {
					defer close(r.done)
					defer r.cancel()
					report := func(v throughputcore.Report) {
						r.mu.Lock()
						r.pending = &v
						r.mu.Unlock()
						r.retry(jobCtx, client, panel, token)
					}
					if j.Role == "prepare" {
						prepareCtx, stop := context.WithDeadline(jobCtx, j.ExpiresAt)
						defer stop()
						c := throughputcore.Prepare(prepareCtx)
						_ = postAgentThroughput(ctx, client, panel, token, "capabilities", c)
						status := "completed"
						if !c.AuthSupported {
							status = "failed"
						}
						report(throughputcore.Report{ID: j.ID, Role: j.Role, Status: status, Error: c.Error})
						return
					}
					throughputcore.Run(jobCtx, j, report)
				}(j, r)
			}
		}
		if time.Since(lastCapability) >= 30*time.Second && len(active) == 0 {
			throughputcore.CleanupStaleAuthDirs(time.Now())
			c := throughputcore.Inspect(ctx)
			_ = postAgentThroughput(ctx, client, panel, token, "capabilities", c)
			lastCapability = time.Now()
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
func fetchAgentThroughputJobs(ctx context.Context, client *http.Client, panel, token string) ([]throughputcore.Job, error) {
	reqCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, panel+"/api/v1/agent/throughput/jobs", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Agent "+token)
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	var payload struct {
		Code string               `json:"code"`
		Data throughputAssignment `json:"data"`
	}
	if err = json.NewDecoder(io.LimitReader(resp.Body, 128*1024)).Decode(&payload); err != nil {
		return nil, err
	}
	if payload.Code != "00000" {
		return nil, fmt.Errorf("throughput poll rejected")
	}
	return payload.Data.Jobs, nil
}
func postAgentThroughputOnce(ctx context.Context, client *http.Client, panel, token, action string, v any) error {
	b, err := json.Marshal(v)
	if err != nil {
		return err
	}
	reqCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, panel+"/api/v1/agent/throughput/"+action, bytes.NewReader(b))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Agent "+token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusConflict {
		return fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	return nil
}
func postAgentThroughput(ctx context.Context, client *http.Client, panel, token, action string, v any) error {
	var last error
	for attempt := 0; attempt < 3; attempt++ {
		last = postAgentThroughputOnce(ctx, client, panel, token, action, v)
		if last == nil {
			return nil
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if attempt < 2 {
			timer := time.NewTimer(time.Duration(attempt+1) * 250 * time.Millisecond)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
		}
	}
	return last
}
