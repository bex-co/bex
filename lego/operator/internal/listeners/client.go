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

package listeners

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"time"
)

// maxAnswerBytes bounds a reply: every port of 1..65535 fits well inside it.
const maxAnswerBytes = 1 << 20

// Client asks a node's reader for a pod's listeners.
type Client struct {
	HTTP *http.Client
}

// NewClient is a Client with its own small connection pool: the operator
// keeps one idle connection to each node's reader instead of a new connection,
// and a new conntrack entry, per read.
func NewClient() Client {
	return Client{HTTP: &http.Client{Transport: &http.Transport{
		DialContext:           (&net.Dialer{Timeout: 3 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		MaxIdleConnsPerHost:   1,
		IdleConnTimeout:       5 * time.Minute,
		ResponseHeaderTimeout: readTimeout + time.Second,
	}}}
}

// Read asks the reader at reader for the ports of the pod with podUID at
// podIP. A 404 is ErrPodNotFound and a 409 ErrAddressMismatch, each wrapped;
// any other failure is an error naming the reader's status.
func (c Client) Read(ctx context.Context, reader netip.AddrPort, podUID, podIP string) ([]int, error) {
	target := url.URL{Scheme: "http", Host: reader.String(), Path: Path,
		RawQuery: url.Values{"pod": {podUID}, "ip": {podIP}}.Encode()}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return nil, err
	}
	httpClient := c.HTTP
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	body := io.LimitReader(resp.Body, maxAnswerBytes)
	defer func() {
		// Read to the end so the connection goes back to the pool.
		_, _ = io.Copy(io.Discard, body)
		_ = resp.Body.Close()
	}()
	switch resp.StatusCode {
	case http.StatusOK:
	case http.StatusNotFound:
		return nil, fmt.Errorf("reader %s: %w", reader, ErrPodNotFound)
	case http.StatusConflict:
		return nil, fmt.Errorf("reader %s: %w", reader, ErrAddressMismatch)
	default:
		return nil, fmt.Errorf("reader %s answered %s", reader, resp.Status)
	}
	var answer Answer
	if err := json.NewDecoder(body).Decode(&answer); err != nil {
		return nil, fmt.Errorf("reader %s: decoding its answer: %w", reader, err)
	}
	for _, port := range answer.Ports {
		if port < 1 || port > 65535 {
			return nil, fmt.Errorf("reader %s answered port %d", reader, port)
		}
	}
	return answer.Ports, nil
}
