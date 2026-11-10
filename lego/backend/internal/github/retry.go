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

package github

import (
	"context"
	"errors"
	"io"
	"math/rand/v2"
	"net/http"
	"time"
)

// githubReadAttempts bounds how often one GitHub read is tried (w8/m150).
// On 2026-11-08 GitHub's API answered a public render.yaml read with 500s and
// reset HTTP/2 streams for minutes, even to outside clients, and a Blueprint
// create made several such reads with no retry; one failure blocked it.
const githubReadAttempts = 3

// retryTransport retries an idempotent GitHub read — a GET or HEAD without a
// body — that failed in transit or answered 500, 502, 503 or 504. It never
// retries a 4xx (429 included: GitHub's rate limit is not transient on this
// scale) or a write, and never waits past the request's own deadline, which
// http.Client.Timeout and the caller's request budget set.
type retryTransport struct {
	base http.RoundTripper
	// wait is the pause before attempt n+1; nil uses a short jittered backoff.
	wait func(attempt int) time.Duration
}

func (t retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if (req.Method != http.MethodGet && req.Method != http.MethodHead) || (req.Body != nil && req.Body != http.NoBody) {
		return t.base.RoundTrip(req)
	}
	for attempt := 1; ; attempt++ {
		resp, err := t.base.RoundTrip(req.Clone(req.Context()))
		if attempt == githubReadAttempts || !retryableRead(resp, err) || req.Context().Err() != nil {
			return resp, err
		}
		if resp != nil {
			_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
			_ = resp.Body.Close()
		}
		timer := time.NewTimer(t.pause(attempt))
		select {
		case <-req.Context().Done():
			timer.Stop()
			return nil, req.Context().Err()
		case <-timer.C:
		}
	}
}

func (t retryTransport) pause(attempt int) time.Duration {
	if t.wait != nil {
		return t.wait(attempt)
	}
	base := time.Duration(attempt) * 250 * time.Millisecond
	return base/2 + rand.N(base)
}

func retryableRead(resp *http.Response, err error) bool {
	if err != nil {
		return !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded)
	}
	switch resp.StatusCode {
	case http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return true
	}
	return false
}
