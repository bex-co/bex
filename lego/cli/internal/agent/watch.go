/*
Copyright 2026.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package agent

import (
	"context"
	"errors"
	"io"
	"net"
	"net/url"
	"time"

	"github.com/render-oss/cli/pkg/command"
)

// Poll pacing. REST has no blocking wait (wait_agent_sessions is MCP-only,
// ADR094 D6), so the CLI reads the session itself: quickly at first, when a
// short turn is likeliest to settle, then backing off. A follow resets to
// pollFirst whenever new transcript parts arrive.
const (
	pollFirst = 2 * time.Second
	pollMax   = 15 * time.Second
	// transientLimit is how many consecutive failed reads a wait survives: a
	// rolling bex-api or a dropped connection should not fail a CI job whose
	// session is still running.
	transientLimit = 5
)

func nextPoll(d time.Duration) time.Duration {
	if d = d * 3 / 2; d > pollMax {
		return pollMax
	}
	return d
}

func settled(s session) bool { return s.State != stateWorking }

// errInterrupted ends a wait the user (Ctrl-C) or the CI runner (SIGTERM)
// stopped. It is not --timeout: that is a deadline, exit 124.
var errInterrupted = command.NewExitError(exitInterrupted, errors.New("interrupted"))

// stopped is why ctx ended a wait: true for the --timeout deadline, which is
// a timedOut result; otherwise the caller returns errInterrupted.
func stopped(ctx context.Context) bool { return errors.Is(ctx.Err(), context.DeadlineExceeded) }

// transient reports a failure worth repeating the request for: a rate limit,
// a gateway hiccup, or the connection failing. The http client wraps every
// failure in *url.Error, which itself satisfies net.Error, so the cause is
// what decides; a malformed URL or a failed token refresh is not transient.
func transient(err error) bool {
	var apiErr *apiError
	if errors.As(err, &apiErr) {
		return apiErr.retryable()
	}
	var urlErr *url.Error
	if errors.As(err, &urlErr) {
		err = urlErr.Err
	}
	var netErr net.Error
	return errors.As(err, &netErr) || errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF)
}

// poller counts consecutive transient read failures across reads and keeps
// the last one's Retry-After for the next sleep.
type poller struct {
	failures int
	backoff  time.Duration
}

// tolerate reports whether err is a transient failure the wait survives.
func (p *poller) tolerate(ctx context.Context, err error) bool {
	if ctx.Err() != nil || !transient(err) || p.failures >= transientLimit {
		return false
	}
	p.failures++
	p.backoff = retryAfter(err, 0)
	return true
}

// sleep waits at least d, longer when the last failure asked for it.
func (p *poller) sleep(ctx context.Context, a *api, d time.Duration) error {
	d, p.backoff = max(d, p.backoff), 0
	return a.sleep(ctx, d)
}

// readSession is a session read that tolerates transientLimit consecutive
// transient failures: ok is false for a tolerated one.
func (a *api) readSession(ctx context.Context, id string, p *poller) (session, []byte, bool, error) {
	s, raw, err := a.session(ctx, id)
	if err == nil {
		p.failures = 0
		return s, raw, true, nil
	}
	if p.tolerate(ctx, err) {
		return session{}, nil, false, nil
	}
	return session{}, nil, false, err
}

// waitResult is where every waited session stood when the wait ended.
type waitResult struct {
	sessions []session
	raws     [][]byte
	timedOut bool
}

// wait polls ids until all settle (first: until one does). ctx carries the
// --timeout deadline; reaching it is a timedOut result, not an error.
func (a *api) wait(ctx context.Context, ids []string, first bool) (waitResult, error) {
	sessions := make([]session, len(ids))
	raws := make([][]byte, len(ids))
	var p poller
	delay := pollFirst
	end := func() (waitResult, error) {
		if stopped(ctx) {
			return collect(sessions, raws, true), nil
		}
		return waitResult{}, errInterrupted
	}
	for {
		done := 0
		for i, id := range ids {
			if raws[i] != nil && settled(sessions[i]) {
				done++
				continue
			}
			s, raw, ok, err := a.readSession(ctx, id, &p)
			if err != nil {
				if ctx.Err() != nil {
					return end()
				}
				return waitResult{}, err
			}
			if ok {
				sessions[i], raws[i] = s, raw
				if settled(s) {
					done++
				}
			}
		}
		if done == len(ids) || (first && done > 0) {
			return collect(sessions, raws, false), nil
		}
		if err := p.sleep(ctx, a, delay); err != nil {
			return end()
		}
		delay = nextPoll(delay)
	}
}

// collect is the sessions read so far; one never read is left out.
func collect(sessions []session, raws [][]byte, timedOut bool) waitResult {
	out := waitResult{timedOut: timedOut}
	for i := range raws {
		if raws[i] != nil {
			out.sessions = append(out.sessions, sessions[i])
			out.raws = append(out.raws, raws[i])
		}
	}
	return out
}

// follow streams the transcript of id from fromTurn on into p until the
// session settles, then returns its final read. A deadline on ctx ends it
// as timedOut. While the agent is quiet, polling backs off to pollMax.
func (a *api) follow(ctx context.Context, id string, fromTurn int, p *partPrinter) (waitResult, error) {
	afterSeq := int64(-1)
	var (
		poll    poller
		last    session
		lastRaw []byte
	)
	end := func() (waitResult, error) {
		if !stopped(ctx) {
			return waitResult{}, errInterrupted
		}
		if lastRaw == nil {
			return waitResult{timedOut: true}, nil
		}
		return waitResult{sessions: []session{last}, raws: [][]byte{lastRaw}, timedOut: true}, nil
	}
	delay := pollFirst
	for {
		n, err := a.drain(ctx, id, &afterSeq, fromTurn, p)
		if err != nil {
			if ctx.Err() != nil {
				return end()
			}
			if !poll.tolerate(ctx, err) {
				return waitResult{}, err
			}
		}
		if n > 0 {
			delay = pollFirst
		} else {
			s, raw, ok, err := a.readSession(ctx, id, &poll)
			if err != nil {
				if ctx.Err() != nil {
					return end()
				}
				return waitResult{}, err
			}
			if ok {
				last, lastRaw = s, raw
				if settled(s) {
					// One more pass: the turn's last parts may land with the
					// state change.
					if _, err := a.drain(ctx, id, &afterSeq, fromTurn, p); err != nil && ctx.Err() == nil {
						return waitResult{}, err
					}
					p.flush()
					return waitResult{sessions: []session{s}, raws: [][]byte{raw}}, nil
				}
			}
		}
		if err := poll.sleep(ctx, a, delay); err != nil {
			return end()
		}
		if n == 0 {
			delay = nextPoll(delay)
		}
	}
}
