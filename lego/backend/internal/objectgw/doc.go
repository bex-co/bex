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

// Package objectgw is the public-bucket gateway of bex object storage (ADR112
// D4): a read-only HTTP server that serves the objects of a bucket a workspace
// admin published with `bex s3 public <bucket> enable`, at
// pub-<xid>.<BEX_BASE_DOMAIN>, to anyone, with no credential.
//
// It is one binary in the backend image (cmd/objectgw), deployed beside the
// static-server and routed to by Traefik. It holds no database credential and
// no write credential: it asks bex-api which bucket a host names, reads the
// object from Wasabi with a read-only identity, and tells bex-api how many bytes
// it sent. verbs.go is bex-api's half of those two calls.
//
// The trust model is the static-server's (ADR029, ADR103): tenant-controlled
// content on a host that shares a registrable domain with tenant apps. The
// gateway serves what was stored, with X-Content-Type-Options: nosniff, and
// claims no stronger isolation.
package objectgw
