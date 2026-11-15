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

// Package notserving writes the responses bex gives a public host it is
// deliberately not serving. The activator and the static-server both answer a
// suspended service, so its page lives here once and cannot drift (w4/669).
package notserving

import (
	"io"
	"mime"
	"net/http"
	"strconv"
	"strings"
)

// WriteSuspended answers a suspended service's public host: 503 plus
// Retry-After, negotiated three ways — a document for browsers, a machine-
// readable object for API clients, plain text for everything else (a bare
// curl, an uptime monitor). It touches nothing: the caller has already decided
// not to serve or wake the App.
//
// Retry-After is an hour, not the wake path's 5 seconds: a suspended service
// comes back when a human resumes it, so telling a monitor to retry every five
// seconds would turn a deliberate suspension into a self-inflicted flood.
func WriteSuspended(w http.ResponseWriter, r *http.Request) {
	Write(w, r, SuspendedRetryAfter, SuspendedPage, SuspendedJSON, SuspendedText)
}

// Write answers a host bex is deliberately not serving: 503 with retryAfter,
// a page for an explicit text/html, JSON for an explicit application/json,
// and plain text for everything else.
func Write(w http.ResponseWriter, r *http.Request, retryAfter, page, json, text string) {
	w.Header().Set("Retry-After", retryAfter)
	negotiate(w, r, http.StatusServiceUnavailable, page, json, text)
}

// negotiate answers status with a page for an explicit text/html, JSON for an
// explicit application/json, and plain text for everything else.
func negotiate(w http.ResponseWriter, r *http.Request, status int, page, json, text string) {
	switch {
	case AcceptsMedia(r.Header.Values("Accept"), "text/html"):
		WriteHTMLPage(w, r, status, page)
	case AcceptsMedia(r.Header.Values("Accept"), "application/json"):
		WriteBody(w, r, status, "application/json", json)
	default:
		WriteBody(w, r, status, "text/plain; charset=utf-8", text)
	}
}

// AcceptsMedia reports whether the Accept header values name want explicitly
// with a non-zero q. Deliberately exact: a wildcard `*/*` (or no Accept at all)
// is NOT a request for any particular representation, so callers fall through
// to their own default rather than guessing a document for a curl.
func AcceptsMedia(values []string, want string) bool {
	for _, value := range values {
		for item := range strings.SplitSeq(value, ",") {
			mediaType, params, err := mime.ParseMediaType(strings.TrimSpace(item))
			if err != nil || !strings.EqualFold(mediaType, want) {
				continue
			}
			if rawQ, ok := params["q"]; ok {
				q, err := strconv.ParseFloat(rawQ, 64)
				if err != nil || q <= 0 {
					continue
				}
			}
			return true
		}
	}
	return false
}

// WriteHTMLPage writes a default page: no-store, HTML, and no body for HEAD.
func WriteHTMLPage(w http.ResponseWriter, r *http.Request, status int, page string) {
	WriteBody(w, r, status, "text/html; charset=utf-8", page)
}

// WriteBody is WriteHTMLPage's non-document sibling: the same no-store,
// HEAD-aware shape for a body whose content type the caller names.
func WriteBody(w http.ResponseWriter, r *http.Request, status int, contentType, body string) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", contentType)
	w.WriteHeader(status)
	if r.Method != http.MethodHead {
		_, _ = io.WriteString(w, body)
	}
}

// The suspended response's Retry-After, JSON object and plain-text body.
const (
	SuspendedRetryAfter = "3600"
	SuspendedJSON       = `{"error":"service suspended"}`
	SuspendedText       = "This service is suspended.\n"
)

// SuspendedPage is the suspended response's document for browsers.
const SuspendedPage = `<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>bex &mdash; Service suspended</title>
  <style>
    body{font-family:system-ui,sans-serif;display:grid;place-items:center;min-height:100vh;margin:0;
      background:#111827;color:#f9fafb}
    main{max-width:36rem;padding:2rem;text-align:center}h1{font-size:1.75rem;margin-bottom:.75rem}
    p{color:#d1d5db;line-height:1.6}
  </style>
</head>
<body><main><h1>This service is suspended.</h1>
<p>Its owner suspended this service, so it is not currently serving traffic.
It will be available again once the owner resumes it.</p></main></body>
</html>`
