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

// Package s3gw is bex's S3-compatible API (ADR112 D3): the REST protocol that
// aws-cli, rclone, boto3 and the AWS SDKs speak, served path style from the
// bex-s3 Deployment at objects.<BEX_BASE_DOMAIN>.
//
// bex terminates S3 itself. Every request is verified with SigV4 against an
// access key whose secret is in OpenBao (objectstorage.AccessKeys), authorized
// as the key's creator on exactly the key's workspace, and, for an object, its
// bytes stream through this process to and from the shared Wasabi bucket, so the
// quota gate, the shadow meters, the audit rows and the limits stay in bex's
// path. The package mounts nothing but S3: no REST, GraphQL, MCP or OAuth route,
// so a bucket named graphql, mcp or v1 can only ever be a bucket.
//
// The files, in the order a request meets them: gateway.go (the entry point and
// the XML error gate), sigv4.go (authentication), router.go (which operation,
// and the 501 for everything bex does not implement).
package s3gw
