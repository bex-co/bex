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

package agentsessions

import (
	"sync"
	"time"
)

const (
	// stopTick is how often the Completer re-reads a session a person just
	// asked to stop: the driver settles within about 3 s (its drain) plus the git
	// facts, so a 2 s look makes Stop show in a few seconds instead of waiting for
	// the 15 s tick that watches every running session.
	stopTick = 2 * time.Second
	// stopWatchWindow bounds how long a stop request keeps the fast look going;
	// past it the ordinary tick is all that is left (the driver's own bounds are
	// 18 s).
	stopWatchWindow = 2 * time.Minute
)

// StopSignal carries "a person asked to stop this session's turn" from
// Service.Interrupt to the Completer in the same process. It only makes the
// Completer look sooner; the request that makes an `interrupted` status
// believable is the database row (w22/m408), so a lost signal, or a request
// served by another replica, costs the ordinary tick and nothing else. A nil
// *StopSignal does nothing.
type StopSignal struct {
	mu      sync.Mutex
	pending map[string]time.Time
}

// Note records a stop request for a session.
func (s *StopSignal) Note(sessionID string, at time.Time) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.pending == nil {
		s.pending = map[string]time.Time{}
	}
	s.pending[sessionID] = at
}

// Pending lists the sessions still worth a fast look at now and forgets the
// ones whose watch window has passed.
func (s *StopSignal) Pending(now time.Time) []string {
	if s == nil {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	ids := make([]string, 0, len(s.pending))
	for id, at := range s.pending {
		if now.Sub(at) > stopWatchWindow {
			delete(s.pending, id)
			continue
		}
		ids = append(ids, id)
	}
	return ids
}

// Done forgets a session whose turn has settled.
func (s *StopSignal) Done(sessionID string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.pending, sessionID)
}
