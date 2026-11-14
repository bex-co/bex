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

// Package openapi holds pinned Render contracts as bytes and nothing else: the
// public OpenAPI document and the MCP result shapes. internal/api verifies the
// OpenAPI digest and serves the request gate from it; internal/rendertest
// validates test responses against the same bytes, which is why the assets
// live in a leaf package both can import.
package openapi

import _ "embed"

// RenderPublicAPI is Render's complete, unversioned public contract, fetched
// from https://api-docs.render.com/openapi/render-public-api-1.json. It is
// deliberately embedded: API startup and CI never depend on Render's
// availability or silently accept an upstream contract change. Provenance and
// the refresh workflow: docs/render-artifacts/openapi-request-validation.md.
//
//go:embed render-public-api-1.json
var RenderPublicAPI []byte

// RenderMCPResults is the result shape of every MCP tool bex shares with
// render-oss/render-mcp-server, captured at the commit the tools pin
// (render-mcp-tools.json) names by scripts/render-mcp-capture.py --results
// (w4/m378): each tool's text form and the JSON keys of the type it marshals.
// internal/rendertest checks bex's MCP answers against it.
//
//go:embed render-mcp-results.json
var RenderMCPResults []byte
