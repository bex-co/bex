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

import "time"

// SetStreamDeadline shortens how long one streamed put or get may run, so a
// test can stall Wasabi without waiting 30 minutes.
func (s *Service) SetStreamDeadline(d time.Duration) { s.streamDeadline = d }

// SetNow fixes the clock the egress budget reads.
func (s *Service) SetNow(now func() time.Time) { s.nowFunc = now }
