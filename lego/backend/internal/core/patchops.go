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

package core

// PatchOps collects the verb calls a patch-shaped adapter should make for the
// arguments a caller actually supplied, in the order they must run. It is the
// shared spine of w1/m71's update_* tools, and it exists so the contract those
// tools advertise is implemented once rather than per resource:
//
//   - an argument the caller omitted adds no op, so it is not written;
//   - a call carrying no settable argument runs no ops at all and reflects
//     current state instead of writing defaults over the resource.
//
// The mirror on the REST side is patchService's ops table (apps/rest.go), whose
// order these tools follow so a multi-field call behaves the same on both
// surfaces.
type PatchOps[T any] struct {
	ops []func() (T, error)
}

// Add queues op when present is true. The op is not run until Run.
func (p *PatchOps[T]) Add(present bool, op func() (T, error)) {
	if present {
		p.ops = append(p.ops, op)
	}
}

// Run applies the queued ops in order and returns the last result. With no ops
// queued it returns current() — the read-only no-op. The first failing op stops
// the chain and returns its error, so a rejected value never reports success.
func (p *PatchOps[T]) Run(current func() (T, error)) (T, error) {
	if len(p.ops) == 0 {
		return current()
	}
	var out T
	for _, op := range p.ops {
		var err error
		if out, err = op(); err != nil {
			var zero T
			return zero, err
		}
	}
	return out, nil
}
