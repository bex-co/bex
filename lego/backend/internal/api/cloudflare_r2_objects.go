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

package api

import (
	"cmp"
	"io"
	"net/http"
	"strconv"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/objectstorage"
)

const cloudflareObjectPattern = cloudflareAccountR2 + "/buckets/{bucket}/objects/{key...}"

// putHeaders are the entity headers a put stores as sent (ADR112 D3).
func putHeaders(r *http.Request) objectstorage.ObjectHeaders {
	return objectstorage.ObjectHeaders{
		ContentType:        r.Header.Get("Content-Type"),
		CacheControl:       r.Header.Get("Cache-Control"),
		ContentDisposition: r.Header.Get("Content-Disposition"),
		ContentEncoding:    r.Header.Get("Content-Encoding"),
		ContentLanguage:    r.Header.Get("Content-Language"),
		Expires:            r.Header.Get("Expires"),
	}
}

func (c *cloudflareR2) registerObjects(mux *http.ServeMux) {
	// The bytes travel in this call, so the server's 30 s timeouts are lifted
	// for it, the Content-Length is required, and the body goes to Wasabi as it
	// arrives (objectstorage.Service.PutStream).
	c.route(mux, "PUT "+cloudflareObjectPattern, objectstorage.AuditVerbPutObject, core.RelCanCreate, func(call *cfCall) error {
		c.extendStreamDeadlines(call.w)
		if class := call.r.Header.Get("cf-r2-storage-class"); class != "" && class != cfStorageClass {
			return cfNotEntitled("the %s storage class is not offered by bex, which has one class", class)
		}
		b, err := call.bucket()
		if err != nil {
			return err
		}
		key := call.r.PathValue("key")
		size := call.r.ContentLength
		etag, err := c.svc.PutStream(call.r.Context(), call.workspaceID, b, key, size, call.r.Body, putHeaders(call.r))
		if err != nil {
			return err
		}
		writeCFResult(call.w, map[string]any{"key": key, "size": size, "etag": etag})
		return nil
	})

	// A get opens the object (a missing one is 10007) and streams it back with
	// the headers it was stored with. A HEAD, which the same pattern matches,
	// asks Wasabi for the headers only.
	c.route(mux, "GET "+cloudflareObjectPattern, objectstorage.AuditVerbGetObject, core.RelCanViewSensitive, func(call *cfCall) error {
		b, err := call.bucket()
		if err != nil {
			return err
		}
		key := call.r.PathValue("key")
		if call.r.Method == http.MethodHead {
			info, err := c.svc.StatObject(call.r.Context(), call.workspaceID, b, key)
			if err != nil {
				return err
			}
			writeObjectHeaders(call.w.Header(), info)
			call.w.WriteHeader(http.StatusOK)
			return nil
		}
		c.extendStreamDeadlines(call.w)
		obj, err := c.svc.OpenObject(call.r.Context(), call.workspaceID, b, key)
		if err != nil {
			return err
		}
		defer obj.Close()
		writeObjectHeaders(call.w.Header(), obj.Info)
		call.w.WriteHeader(http.StatusOK)
		// A failure part-way cannot become an envelope: the status is sent. The
		// client sees a short body against Content-Length.
		_, _ = io.Copy(call.w, obj)
		return nil
	})

	// A delete of a missing key succeeds, as S3 and Cloudflare answer; Render's
	// route answers 404 and wrangler treats a 404 here as success anyway.
	c.route(mux, "DELETE "+cloudflareObjectPattern, objectstorage.AuditVerbDeleteObject, core.RelCanCreate, func(call *cfCall) error {
		b, err := call.bucket()
		if err != nil {
			return err
		}
		if err := c.svc.RemoveObject(call.r.Context(), call.workspaceID, b, call.r.PathValue("key")); err != nil {
			return err
		}
		writeCFResult(call.w, nil)
		return nil
	})
}

// writeObjectHeaders sets the response headers of an object read. The stored
// entity headers come back as stored. Tenant content is served from bex's own
// origin, so the response also tells a browser not to render it: a signed-in
// browser that followed a link here would otherwise run a tenant's HTML with
// its session (ADR095 D7's rule for attachments, kept here).
func writeObjectHeaders(h http.Header, info objectstorage.ObjectInfo) {
	set := func(name, value string) {
		if value != "" {
			h.Set(name, value)
		}
	}
	set("Content-Type", info.Headers.ContentType)
	set("Cache-Control", info.Headers.CacheControl)
	set("Content-Encoding", info.Headers.ContentEncoding)
	set("Content-Language", info.Headers.ContentLanguage)
	set("Expires", info.Headers.Expires)
	set("ETag", info.ETag)
	if !info.LastModified.IsZero() {
		h.Set("Last-Modified", info.LastModified.UTC().Format(http.TimeFormat))
	}
	h.Set("Content-Disposition", cmp.Or(info.Headers.ContentDisposition, "attachment"))
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Content-Security-Policy", "default-src 'none'; sandbox")
	h.Set("Content-Length", strconv.FormatInt(info.Size, 10))
}
