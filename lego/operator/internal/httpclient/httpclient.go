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

// Package httpclient owns the operator's bounded, pooled HTTP dependency
// policy. Registry and Prometheus calls share one transport instead of creating
// unbounded default clients or inheriting the net/http zero-timeout defaults.
package httpclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"time"
)

const (
	RequestTimeout = 5 * time.Second
	MaxBodyBytes   = 1 << 20
)

// Shared is safe for concurrent use and reuses idle connections across every
// reconciler. Tests may still inject a client at the owning seam.
var Shared = New(RequestTimeout)

func New(totalTimeout time.Duration) *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			Proxy: http.ProxyFromEnvironment,
			DialContext: (&net.Dialer{
				Timeout:   2 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			ForceAttemptHTTP2:     true,
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   10,
			IdleConnTimeout:       60 * time.Second,
			TLSHandshakeTimeout:   2 * time.Second,
			ResponseHeaderTimeout: 3 * time.Second,
			ExpectContinueTimeout: time.Second,
		},
		Timeout: totalTimeout,
	}
}

// WithTimeout adds the operator's total request deadline while preserving an
// earlier caller deadline or cancellation.
func WithTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, RequestTimeout)
}

func ReadAll(body io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(body, MaxBodyBytes+1))
	if err != nil {
		return nil, err
	}
	if len(data) > MaxBodyBytes {
		return nil, fmt.Errorf("HTTP response body exceeds %d bytes", MaxBodyBytes)
	}
	return data, nil
}

func DecodeJSON(body io.Reader, dst any) error {
	data, err := ReadAll(body)
	if err != nil {
		return err
	}
	if err := json.Unmarshal(data, dst); err != nil {
		return err
	}
	return nil
}
