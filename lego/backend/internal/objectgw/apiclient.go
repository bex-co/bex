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

package objectgw

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"time"
)

// bexAPITimeout bounds one call to bex-api.
const bexAPITimeout = 10 * time.Second

// APIClient is the gateway's client of bex-api's internal listener: the URL, the
// bearer both verbs share, and an optional HTTP client (tests give their own).
type APIClient struct {
	URL    string
	Token  string
	Client *http.Client
}

// defaultAPIClient keeps a few connections to bex-api open: lookups run up to
// LookupsPerSecond, and the stdlib default of two idle connections per host would
// handshake on every burst.
var defaultAPIClient = &http.Client{Timeout: bexAPITimeout, Transport: func() *http.Transport {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.MaxIdleConnsPerHost = 16
	return t
}()}

// do sends one authenticated request to path and returns the response; the
// caller closes its body. The error never names the URL.
func (c *APIClient) do(ctx context.Context, method, path string, body []byte) (*http.Response, error) {
	var rdr io.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.URL+path, rdr)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	client := c.Client
	if client == nil {
		client = defaultAPIClient
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, errors.New("bex-api is unreachable")
	}
	return resp, nil
}

// drainClose reads what is left of a small body so the connection is reused.
func drainClose(resp *http.Response) {
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	resp.Body.Close()
}
