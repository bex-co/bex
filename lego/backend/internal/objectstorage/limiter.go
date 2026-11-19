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

package objectstorage

import (
	"errors"
	"sync"
)

var (
	// ErrTooManyStreams refuses a stream while the workspace, or the replica,
	// already has its maximum in flight.
	ErrTooManyStreams = errors.New("too many concurrent object streams")
	// ErrRateLimited refuses a presigned-URL mint past the workspace's rate.
	ErrRateLimited = errors.New("too many object requests")
)

// StreamLimiter bounds the uploads and downloads in flight through bex-api:
// perWorkspace for each workspace and perReplica for all of them together.
type StreamLimiter struct {
	mu           sync.Mutex
	perWorkspace int
	perReplica   int
	total        int
	active       map[string]int
}

// NewStreamLimiter returns a limiter with the two caps.
func NewStreamLimiter(perWorkspace, perReplica int) *StreamLimiter {
	return &StreamLimiter{perWorkspace: perWorkspace, perReplica: perReplica, active: map[string]int{}}
}

// Acquire takes a slot for workspaceID, or returns ErrTooManyStreams. The
// release function gives the slot back and is safe to call more than once.
func (l *StreamLimiter) Acquire(workspaceID string) (release func(), err error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.total >= l.perReplica || l.active[workspaceID] >= l.perWorkspace {
		return nil, ErrTooManyStreams
	}
	l.total++
	l.active[workspaceID]++
	var once sync.Once
	return func() {
		once.Do(func() {
			l.mu.Lock()
			defer l.mu.Unlock()
			l.total--
			if l.active[workspaceID]--; l.active[workspaceID] <= 0 {
				delete(l.active, workspaceID)
			}
		})
	}, nil
}

// Load is the streams in flight on this replica and the replica's cap.
func (l *StreamLimiter) Load() (inFlight, limit int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.total, l.perReplica
}

// StreamLoad is the streams in flight on this replica and its cap, for the saturation alert.
func (s *Service) StreamLoad() (inFlight, limit int) { return s.streams.Load() }

// AcquireStream takes a stream slot for the workspace.
func (s *Service) AcquireStream(workspaceID string) (release func(), err error) {
	return s.streams.Acquire(workspaceID)
}

// AllowMint spends one token of the workspace's presigned-URL mint rate, or
// returns ErrRateLimited.
func (s *Service) AllowMint(workspaceID string) error {
	if !s.mints.Bucket(workspaceID).Allow() {
		return ErrRateLimited
	}
	return nil
}
