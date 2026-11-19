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

package s3gw

import (
	"net/http"
	"strings"
)

// A header whose semantics bex does not implement answers 501 and is never
// ignored (ADR112 D3): a client that sent it expects what it asks for, and a put
// that drops If-None-Match (Terraform's state lock uses one) or a server-side
// encryption request would be a silent lie.

// refuseUnimplementedHeaders applies the refusals to a request that stores
// something. If-Match and If-None-Match are the conditions of a read on a get and
// a head, which bex serves.
func refuseUnimplementedHeaders(r *http.Request, rt *route) *Error {
	if !rt.write {
		return nil
	}
	for name, values := range r.Header {
		lower := strings.ToLower(name)
		switch {
		case lower == "if-none-match", lower == "if-match":
			return errNotImplemented("Conditional writes (" + name + ") are not implemented")
		case lower == "x-amz-tagging", lower == "x-amz-tagging-directive":
			return errNotImplemented("Object tags are not implemented")
		case strings.HasPrefix(lower, "x-amz-server-side-encryption"):
			return errNotImplemented("Server-side encryption settings are not implemented; objects are encrypted at rest by the store")
		case strings.HasPrefix(lower, "x-amz-grant-"):
			return errNotImplemented("ACLs are not implemented; publish a bucket with `bex s3 public`")
		case lower == "x-amz-bucket-object-lock-enabled":
			// rclone sends `false` on every CreateBucket, which is S3's own default: it asks for
			// nothing, so it is served (found by scripts/s3-contract.sh, w21/m499 t004).
			for _, v := range values {
				if !strings.EqualFold(v, "false") {
					return errNotImplemented("Object lock is not implemented")
				}
			}
		case strings.HasPrefix(lower, "x-amz-object-lock-"):
			return errNotImplemented("Object lock is not implemented")
		case lower == "x-amz-website-redirect-location":
			return errNotImplemented("Website redirects are not implemented")
		case lower == "x-amz-storage-class":
			for _, v := range values {
				if v != "STANDARD" {
					return errNotImplemented("Storage classes are not implemented; only STANDARD is")
				}
			}
		case lower == "x-amz-acl":
			for _, v := range values {
				if v != "private" {
					return errNotImplemented("ACLs are not implemented; every object is private, and a bucket is published with `bex s3 public`")
				}
			}
		}
	}
	return nil
}
