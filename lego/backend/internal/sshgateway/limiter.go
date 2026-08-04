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

package sshgateway

import "sync"

// SessionLimiter enforces the gateway-wide global and per-identity session
// caps. One instance is shared by every transport (native SSH, Browser Web
// Shell, sandbox exec) so the caps bound the process, not each feature.
type SessionLimiter struct {
	maxSessions    int
	maxPerIdentity int

	mu          sync.Mutex
	global      int
	perIdentity map[string]int
}

// NewSessionLimiter builds a limiter; non-positive values take the defaults
// (100 global, 5 per identity — BEX_SSH_MAX_SESSIONS[_PER_IDENTITY]).
func NewSessionLimiter(maxSessions, maxPerIdentity int) *SessionLimiter {
	if maxSessions <= 0 {
		maxSessions = 100
	}
	if maxPerIdentity <= 0 {
		maxPerIdentity = 5
	}
	return &SessionLimiter{
		maxSessions:    maxSessions,
		maxPerIdentity: maxPerIdentity,
		perIdentity:    map[string]int{},
	}
}

// Acquire reserves one session slot for subject. On refusal the second return
// names the exhausted scope: "global" or "identity".
func (l *SessionLimiter) Acquire(subject string) (bool, string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.global >= l.maxSessions {
		return false, "global"
	}
	if l.perIdentity[subject] >= l.maxPerIdentity {
		return false, "identity"
	}
	l.global++
	l.perIdentity[subject]++
	return true, ""
}

// Release returns subject's slot acquired by a successful Acquire.
func (l *SessionLimiter) Release(subject string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.global--
	l.perIdentity[subject]--
	if l.perIdentity[subject] == 0 {
		delete(l.perIdentity, subject)
	}
}
