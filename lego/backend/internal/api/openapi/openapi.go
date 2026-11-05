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

// Package openapi holds the pinned Render public OpenAPI document as bytes and
// nothing else. internal/api verifies its digest and serves the request gate
// from it; internal/rendertest validates test responses against the same
// bytes, which is why the asset lives in a leaf package both can import.
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
