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

package objectstorage

import (
	"net/http"
	"strings"
	"time"
)

// ConditionStatus evaluates the four conditional fields of a read, or of a copy's source,
// against an object as S3 does, in RFC 7232's order: 412 when If-Match or
// If-Unmodified-Since fails, 304 when If-None-Match matches or, without it,
// If-Modified-Since says the object is unchanged, 0 when the object is to be served. A
// copy treats 304 as a failed precondition too (412).
func (o GetOptions) ConditionStatus(etag string, lastModified time.Time) int {
	if o.IfMatch != "" && !etagListHas(o.IfMatch, etag) {
		return http.StatusPreconditionFailed
	}
	if !o.IfUnmodifiedSince.IsZero() && lastModified.After(o.IfUnmodifiedSince) {
		return http.StatusPreconditionFailed
	}
	if o.IfNoneMatch != "" {
		if etagListHas(o.IfNoneMatch, etag) {
			return http.StatusNotModified
		}
		return 0
	}
	if !o.IfModifiedSince.IsZero() && !lastModified.After(o.IfModifiedSince) {
		return http.StatusNotModified
	}
	return 0
}

// etagListHas reports whether a comma-separated list of entity tags, as a client sends in
// If-Match, holds etag or "*". Quotes and the weak prefix W/ are not part of the compare.
func etagListHas(list, etag string) bool {
	for _, tag := range strings.Split(list, ",") {
		if tag = strings.TrimSpace(tag); tag == "*" || etagEqual(tag, etag) {
			return true
		}
	}
	return false
}

// etagEqual compares two entity tags without their quotes or a weak prefix.
func etagEqual(a, b string) bool {
	norm := func(s string) string { return strings.Trim(strings.TrimPrefix(strings.TrimSpace(s), "W/"), `"`) }
	return norm(a) == norm(b)
}
