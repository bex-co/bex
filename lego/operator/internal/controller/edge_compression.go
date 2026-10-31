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

package controller

import (
	"context"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// edgeCompressedTypes are the response media types the edge compresses, as
// Render's load balancer compresses app responses (w8/m83): text,
// script, style and structured-data types. Images, media, archives and fonts
// are already compressed, and text/event-stream stays untouched so an event
// stream is exactly what the service wrote. Traefik matches the media type
// with its parameters ignored.
var edgeCompressedTypes = []any{
	"text/html", "text/plain", "text/css", "text/javascript", "text/xml", "text/csv", "text/markdown",
	"application/javascript", "application/x-javascript", "application/json", "application/ld+json",
	"application/manifest+json", "application/problem+json", "application/graphql-response+json",
	"application/x-ndjson", "application/xml", "application/xhtml+xml", "application/rss+xml",
	"application/atom+xml", "application/wasm", "image/svg+xml",
}

// edgeCompressionMiddlewares is the router chain that compresses a web
// service's public responses: Traefik's own compress, gzip only, for at least
// 1 KiB of a listed type and never an already encoded or HEAD response. Brotli
// is left off because Traefik v3.7.5's Brotli writer encodes a 206 Range body
// while Content-Range still counts identity bytes; its gzip writer passes a
// partial response through untouched. Two differences from Render remain,
// signed off by the user in ADR018: a Cache-Control: no-transform response is
// still compressed, and a strong ETag stays strong on the encoded body (w8/m83).
func edgeCompressionMiddlewares(app *appv1alpha1.App) (names []string, specs []map[string]any) {
	return []string{app.Name + "-compress"}, []map[string]any{
		{"compress": map[string]any{"encodings": []any{"gzip"}, "includedContentTypes": edgeCompressedTypes}},
	}
}

// reconcileEdgeCompressionMiddlewares creates (enabled) or removes the App's
// edge compression chain and returns its names in router order.
func (r *AppReconciler) reconcileEdgeCompressionMiddlewares(ctx context.Context, app *appv1alpha1.App, enabled bool) ([]string, error) {
	names, specs := edgeCompressionMiddlewares(app)
	for i, name := range names {
		if !enabled {
			if err := deleteOwned(ctx, r.Client, app, traefikHTTPMiddlewareGVK, name); err != nil {
				return nil, err
			}
			continue
		}
		if err := upsertOwned(ctx, r.Client, r.Scheme, app, traefikHTTPMiddlewareGVK, name, specs[i]); err != nil {
			return nil, err
		}
	}
	if !enabled {
		return nil, nil
	}
	return names, nil
}
