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

import (
	"sync"
	"time"
)

// SessionLimiter enforces the gateway-wide global and per-identity session
// caps. One instance is shared by native SSH, the Browser Web Shell and agent
// attach so the caps bound the process, not each feature. Sandbox exec and
// file streams hold SandboxStreamLimits instead.
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
	releaseSlot(&l.global, l.perIdentity, subject)
}

// releaseSlot returns one of subject's held slots. A release for a subject
// holding none is ignored: an unbalanced release must not drive the counts
// negative, which would lift both caps by one until the process restarts.
func releaseSlot(global *int, perIdentity map[string]int, subject string) {
	if perIdentity[subject] == 0 {
		return
	}
	*global--
	perIdentity[subject]--
	if perIdentity[subject] == 0 {
		delete(perIdentity, subject)
	}
}

// The pools of SandboxStreamLimits, as bex_ssh_gateway_active_sandbox_streams
// labels them.
const (
	sandboxPoolCaller   = "caller"
	sandboxPoolPlatform = "platform"
)

// SandboxStreamLimits bounds the gateway's sandbox exec and file streams
// apart from the SessionLimiter of native SSH, the web shell and agent attach
// (w22/m623), so sandbox commands neither meet that limiter's 5 per identity
// nor take its slots. Render documents no concurrency limit on commands or
// transfers, only per-minute rates (render.com/docs/sandboxes-limits), so a
// caller's streams are held only to what one process can carry, unless
// BEX_SANDBOX_MAX_STREAMS_PER_IDENTITY is set lower. Streams bex-api opens
// for itself (the agent-session Completer's reads, the command probe, an agent
// session's file copies) take a pool of their own: callers cannot starve them,
// and they cannot starve callers.
type SandboxStreamLimits struct {
	callers, platform *SessionLimiter
}

// NewSandboxStreamLimits builds both pools. Non-positive values take the
// defaults: 100 caller streams, per identity as many as the pool holds, and
// 32 platform streams, 5 per key (BEX_SANDBOX_MAX_STREAMS[_PER_IDENTITY],
// BEX_SANDBOX_MAX_PLATFORM_STREAMS[_PER_KEY]).
func NewSandboxStreamLimits(maxStreams, maxPerIdentity, maxPlatform, maxPlatformPerKey int) *SandboxStreamLimits {
	if maxStreams <= 0 {
		maxStreams = 100
	}
	if maxPerIdentity <= 0 {
		maxPerIdentity = maxStreams
	}
	if maxPlatform <= 0 {
		maxPlatform = 32
	}
	if maxPlatformPerKey <= 0 {
		maxPlatformPerKey = 5
	}
	return &SandboxStreamLimits{
		callers:  NewSessionLimiter(maxStreams, maxPerIdentity),
		platform: NewSessionLimiter(maxPlatform, maxPlatformPerKey),
	}
}

// Admit reserves a stream slot for key, from the platform pool when platform
// is set (a ticket bex-api minted for itself), and reports the stream on m as
// held. It returns the stream's end, which frees the slot and records how the
// stream ended, or ok false once it has counted the refusal by scope
// (sandbox_global, sandbox_identity, sandbox_platform_global,
// sandbox_platform_identity).
func (l *SandboxStreamLimits) Admit(m *Metrics, key string, platform bool) (end func(result string), ok bool) {
	pool, limiter, scopePrefix := sandboxPoolCaller, l.callers, "sandbox_"
	if platform {
		pool, limiter, scopePrefix = sandboxPoolPlatform, l.platform, "sandbox_platform_"
	}
	if acquired, scope := limiter.Acquire(key); !acquired {
		m.LimitRejected(scopePrefix + scope)
		return nil, false
	}
	started := time.Now()
	m.sandboxStreamStarted(pool)
	var once sync.Once
	return func(result string) {
		once.Do(func() {
			limiter.Release(key)
			m.sandboxStreamEnded(pool, result, time.Since(started))
		})
	}, true
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
	releaseSlot(&l.global, l.perIdentity, subject)
}

// SourceLimiter bounds concurrent in-flight entries per SOURCE address. The
// native-SSH pre-auth pool is global (MaxPreAuthConns), and a global-only cap
// admits admission starvation: one unauthenticated source holding every slot
// with silent connections forces the gateway to shed everyone else's handshakes
// for the full HandshakeTimeout (round-11 #2). The per-source cap is acquired
// after the PROXY protocol header resolves the real client (Traefik asserts it;
// an untrusted peer keys on its own address), so one source — however many
// sockets it opens — can only ever occupy maxPerSource of the global pool. The
// same shape the pg/kv SNI proxies and the git/model proxies already enforce.
type SourceLimiter struct {
	maxPerSource int

	mu    sync.Mutex
	count map[string]int
}

// NewSourceLimiter builds a per-source limiter; a non-positive value takes the
// default (32 — BEX_SSH_MAX_PREAUTH_CONNS_PER_SOURCE). 0 never happens via the
// env wiring (the default), but -1 explicitly disables per-source fairness,
// restoring the global-only behavior.
func NewSourceLimiter(maxPerSource int) *SourceLimiter {
	if maxPerSource == 0 {
		maxPerSource = 32
	}
	return &SourceLimiter{
		maxPerSource: maxPerSource,
		count:        map[string]int{},
	}
}

// Acquire reserves one slot for source. false means the source is at its cap.
func (l *SourceLimiter) Acquire(source string) bool {
	if l.maxPerSource < 0 {
		return true // disabled
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.count[source] >= l.maxPerSource {
		return false
	}
	l.count[source]++
	return true
}

// Release returns source's slot acquired by a successful Acquire.
func (l *SourceLimiter) Release(source string) {
	if l.maxPerSource < 0 {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l.count[source]--
	if l.count[source] <= 0 {
		delete(l.count, source)
	}
}
