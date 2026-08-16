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

// ChannelLimiter bounds concurrent exec STREAMS (SSH session channels), not
// transports. The session limiter counts connections, but one agent-session
// connection multiplexes many pods/exec streams (ADR054 D3), so transport caps
// alone let one identity fan out to connections × channels streams (codex
// round-8 #7: 5 × 16 = 80 pods/exec streams at the defaults). Every accepted
// session channel — single- and multi-channel paths alike — acquires a slot
// here and releases it when its exec ends. The single-exec-per-connection
// transports (webshell, sandboxsse, agentattach) are bounded by the session
// limiter instead: their one exec IS their session slot.
type ChannelLimiter struct {
	maxGlobal      int
	maxPerIdentity int

	mu          sync.Mutex
	global      int
	perIdentity map[string]int
}

// NewChannelLimiter builds a channel limiter; non-positive values take the
// defaults (512 global, 32 per identity — two fully-multiplexed connections'
// worth — BEX_SSH_MAX_CHANNELS[_PER_IDENTITY]).
func NewChannelLimiter(maxGlobal, maxPerIdentity int) *ChannelLimiter {
	if maxGlobal <= 0 {
		maxGlobal = 512
	}
	if maxPerIdentity <= 0 {
		maxPerIdentity = 32
	}
	return &ChannelLimiter{
		maxGlobal:      maxGlobal,
		maxPerIdentity: maxPerIdentity,
		perIdentity:    map[string]int{},
	}
}

// AcquireChannel reserves one exec-stream slot for subject. On refusal the
// second return names the exhausted scope: "channel_global" or
// "channel_identity".
func (l *ChannelLimiter) AcquireChannel(subject string) (bool, string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.global >= l.maxGlobal {
		return false, "channel_global"
	}
	if l.perIdentity[subject] >= l.maxPerIdentity {
		return false, "channel_identity"
	}
	l.global++
	l.perIdentity[subject]++
	return true, ""
}

// ReleaseChannel returns subject's slot acquired by a successful AcquireChannel.
func (l *ChannelLimiter) ReleaseChannel(subject string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.global--
	l.perIdentity[subject]--
	if l.perIdentity[subject] == 0 {
		delete(l.perIdentity, subject)
	}
}
