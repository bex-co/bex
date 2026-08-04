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
	"context"

	"k8s.io/client-go/tools/remotecommand"
)

// ClampExit bounds a Kubernetes exit code to a byte, matching SSH exit-status
// semantics; every transport reports the same clamped value.
func ClampExit(code int) int {
	if code < 0 {
		return 255
	}
	if code > 255 {
		return 255
	}
	return code
}

// ValidTerminalSize reports whether the dimensions fit Kubernetes' uint16
// terminal-size fields.
func ValidTerminalSize(columns, rows uint32) bool {
	return columns > 0 && columns <= uint32(^uint16(0)) && rows > 0 && rows <= uint32(^uint16(0))
}

// SizeQueue adapts pushed terminal resizes to remotecommand.TerminalSizeQueue.
type SizeQueue struct {
	ctx context.Context
	ch  chan remotecommand.TerminalSize
}

func NewSizeQueue(ctx context.Context, initial *remotecommand.TerminalSize) *SizeQueue {
	q := &SizeQueue{ctx: ctx, ch: make(chan remotecommand.TerminalSize, 8)}
	if initial != nil {
		q.Push(*initial)
	}
	return q
}

func (q *SizeQueue) Push(size remotecommand.TerminalSize) {
	select {
	case q.ch <- size:
	default:
		// Keep resize handling bounded under a client flood. Once the executor
		// drains the queue, later resize events can be accepted again.
	}
}

func (q *SizeQueue) Next() *remotecommand.TerminalSize {
	select {
	case <-q.ctx.Done():
		return nil
	case size := <-q.ch:
		return &size
	}
}
