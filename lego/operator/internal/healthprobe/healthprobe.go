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

// Package healthprobe is the no-redirect HTTP health check the operator
// projects as an exec probe (w8/m133). Render counts a "2xx or 3xx status code
// within five seconds" as healthy; the kubelet's HTTP prober instead follows
// every same-host redirect (every relative Location) up to ten hops and scores
// the final response, so a health path that answers 302 never became Ready.
// This check makes one request, follows nothing, and scores that response.
package healthprobe

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"
)

// Healthy reports whether an HTTP status is a passing health check: 2xx or 3xx.
func Healthy(status int) bool { return status >= 200 && status < 400 }

// Check makes one GET to http://addr:port<path> with Host header host (empty:
// the URL's own), never following a redirect, and returns nil when the status
// is healthy. The request is bounded by timeout.
func Check(ctx context.Context, addr string, port int, path, host string, timeout time.Duration) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	target := url.URL{Scheme: "http", Host: net.JoinHostPort(addr, strconv.Itoa(port))}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return err
	}
	// The path is passed through as authored, query string included.
	if rel, err := url.Parse(path); err == nil {
		req.URL.Path, req.URL.RawPath, req.URL.RawQuery = rel.Path, rel.RawPath, rel.RawQuery
	}
	if host != "" {
		req.Host = host
	}
	client := &http.Client{
		Transport:     &http.Transport{DisableKeepAlives: true},
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 64<<10))
	if !Healthy(resp.StatusCode) {
		return fmt.Errorf("health check answered %d", resp.StatusCode)
	}
	return nil
}

// PodAddrs lists this pod's non-loopback unicast addresses, IPv4 first: the
// address the kubelet's HTTP probe would have dialed, so an app that listens
// only on its pod IP, or on all interfaces, is checked exactly as before.
func PodAddrs() []string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return nil
	}
	var v4, v6 []string
	for _, a := range addrs {
		ipnet, ok := a.(*net.IPNet)
		if !ok || !ipnet.IP.IsGlobalUnicast() {
			continue
		}
		if ipnet.IP.To4() != nil {
			v4 = append(v4, ipnet.IP.String())
		} else {
			v6 = append(v6, ipnet.IP.String())
		}
	}
	return append(v4, v6...)
}

// Install copies the running executable to dest (mode 0555) through a
// temporary sibling, so a pod restart never leaves a half-written binary where
// the probe is exec'd. The init container runs this against the shared volume.
func Install(dest string) error {
	src, err := os.Executable()
	if err != nil {
		return err
	}
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = in.Close() }()
	tmp, err := os.CreateTemp(filepath.Dir(dest), ".health-probe-*")
	if err != nil {
		return err
	}
	// Removing the temporary name after a successful rename is a no-op.
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err := io.Copy(tmp, in); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o555); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), dest)
}
