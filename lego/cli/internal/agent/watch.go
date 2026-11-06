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
	"net"
	"time"
)

// Poll pacing. REST has no blocking wait (wait_agent_sessions is MCP-only,
// ADR094 D6), so the CLI reads the session itself: quickly at first, when a
// short turn is likeliest to settle, then backing off.
const (
	pollFirst = 2 * time.Second
	pollMax   = 15 * time.Second
	// followPoll is the transcript cadence while output is streaming.
	followPoll = 2 * time.Second
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

// transient reports a read failure worth retrying while waiting.
func transient(err error) bool {
	var apiErr *apiError
	if errors.As(err, &apiErr) {
		return apiErr.retryable()
	}
	var netErr net.Error
	return errors.As(err, &netErr) || errors.Is(err, context.DeadlineExceeded)
}

// readSession is a session read that tolerates transientLimit consecutive
// transient failures; failures counts them across calls.
func (a *api) readSession(ctx context.Context, id string, failures *int) (session, []byte, bool, error) {
	s, raw, err := a.session(ctx, id)
	if err == nil {
		*failures = 0
		return s, raw, true, nil
	}
	if ctx.Err() == nil && transient(err) && *failures < transientLimit {
		*failures++
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
	failures := 0
	delay := pollFirst
	for {
		done := 0
		for i, id := range ids {
			if raws[i] != nil && settled(sessions[i]) {
				done++
				continue
			}
			s, raw, ok, err := a.readSession(ctx, id, &failures)
			if err != nil {
				if ctx.Err() != nil {
					return a.finish(ids, sessions, raws), nil
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
			return waitResult{sessions: sessions, raws: raws}, nil
		}
		if err := a.sleep(ctx, delay); err != nil {
			return a.finish(ids, sessions, raws), nil
		}
		delay = nextPoll(delay)
	}
}

// finish is a timed-out wait; a session never read is left out.
func (a *api) finish(ids []string, sessions []session, raws [][]byte) waitResult {
	out := waitResult{timedOut: true}
	for i := range ids {
		if raws[i] != nil {
			out.sessions = append(out.sessions, sessions[i])
			out.raws = append(out.raws, raws[i])
		}
	}
	return out
}

// follow streams the transcript of id from fromTurn on into p until the
// session settles, then returns its final read. A deadline on ctx ends it
// as timedOut.
func (a *api) follow(ctx context.Context, id string, fromTurn int, p *partPrinter) (waitResult, error) {
	afterSeq := int64(-1)
	failures := 0
	var last session
	var lastRaw []byte
	drain := func() (int, error) {
		n := 0
		for {
			page, err := a.transcript(ctx, id, afterSeq)
			if err != nil {
				return n, err
			}
			p.turns(page.Turns)
			for _, part := range page.Parts {
				if part.Turn >= fromTurn {
					p.add(part)
				}
			}
			n += len(page.Parts)
			if len(page.Parts) == 0 || page.NextAfterSeq <= afterSeq {
				return n, nil
			}
			afterSeq = page.NextAfterSeq
		}
	}
	for {
		n, err := drain()
		if err != nil {
			if ctx.Err() != nil {
				return a.timedOutFollow(last, lastRaw), nil
			}
			if !transient(err) || failures >= transientLimit {
				return waitResult{}, err
			}
			failures++
		}
		if n == 0 {
			s, raw, ok, err := a.readSession(ctx, id, &failures)
			if err != nil {
				if ctx.Err() != nil {
					return a.timedOutFollow(last, lastRaw), nil
				}
				return waitResult{}, err
			}
			if ok {
				last, lastRaw = s, raw
				if settled(s) {
					// One more pass: the turn's last parts may land with the
					// state change.
					if _, err := drain(); err != nil && ctx.Err() == nil {
						return waitResult{}, err
					}
					p.flush()
					return waitResult{sessions: []session{s}, raws: [][]byte{raw}}, nil
				}
			}
		}
		if err := a.sleep(ctx, followPoll); err != nil {
			return a.timedOutFollow(last, lastRaw), nil
		}
	}
}

func (a *api) timedOutFollow(last session, raw []byte) waitResult {
	if raw == nil {
		return waitResult{timedOut: true}
	}
	return waitResult{sessions: []session{last}, raws: [][]byte{raw}, timedOut: true}
}
