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
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/objectstorage"
	"github.com/bex-co/bex/lego/backend/internal/store"
)

// The object operations (ADR112 D3): put, get, head, delete, batch delete, copy and the
// tagging stub. A put streams to a server-signed Wasabi PUT that signs its length and
// metadata, so nothing is spooled; a get opens the object once and streams it back.

func init() {
	handlers["PutObject"] = (*request).putObject
	handlers["GetObject"] = (*request).getObject
	handlers["HeadObject"] = (*request).headObject
	handlers["DeleteObject"] = (*request).deleteObject
	handlers["DeleteObjects"] = (*request).deleteObjects
	handlers["GetObjectTagging"] = (*request).getObjectTagging
}

// copySourceHeaderPrefix starts the headers a copy sends about its source.
const copySourceHeaderPrefix = "X-Amz-Copy-Source-"

const (
	// maxMetadataBytes is S3's limit on the user metadata of an object (2 KiB).
	maxMetadataBytes = 2048
	// maxDeleteBody bounds a DeleteObjects document: 1,000 keys of 1,024 bytes, escaped.
	maxDeleteBody = 4 << 20
)

// extendDeadlines gives a stream the stream deadline instead of the server's short ones.
func (rq *request) extendDeadlines() {
	rc := http.NewResponseController(rq.w)
	deadline := rq.g.cfg.Now().Add(objectstorage.StreamDeadline)
	_ = rc.SetReadDeadline(deadline)
	_ = rc.SetWriteDeadline(deadline)
}

// entityHeaders are what a put or a copy with the REPLACE directive stores: the entity
// headers as sent (the aws-chunked framing token removed from Content-Encoding) and
// the user metadata.
func (rq *request) entityHeaders() (objectstorage.ObjectHeaders, *Error) {
	h := rq.r.Header
	out := objectstorage.ObjectHeaders{
		ContentType: h.Get("Content-Type"), CacheControl: h.Get("Cache-Control"), ContentDisposition: h.Get("Content-Disposition"),
		ContentEncoding: storedEncoding(h.Values("Content-Encoding")), ContentLanguage: h.Get("Content-Language"), Expires: h.Get("Expires"),
	}
	total := 0
	for name, values := range h {
		lower := strings.ToLower(name)
		if !strings.HasPrefix(lower, "x-amz-meta-") {
			continue
		}
		key := strings.TrimPrefix(lower, "x-amz-meta-")
		value := strings.Join(values, ",")
		total += len(key) + len(value)
		if key == "" || strings.ContainsAny(value, "\r\n") {
			return out, errInvalidArgument("Metadata names must not be empty and values must not contain line breaks.")
		}
		if out.Metadata == nil {
			out.Metadata = map[string]string{}
		}
		out.Metadata[key] = value
	}
	if total > maxMetadataBytes {
		return out, newError(http.StatusBadRequest, "MetadataTooLarge", "Your metadata headers exceed the maximum allowed metadata size.")
	}
	return out, nil
}

func (rq *request) putObject() *Error {
	if isCopy(rq.r) {
		return rq.copyObject()
	}
	b, e := rq.bucketRow()
	if e != nil {
		return e
	}
	headers, e := rq.entityHeaders()
	if e != nil {
		return e
	}
	body, e := rq.openBody(maxSingleObject)
	if e != nil {
		return e
	}
	rq.extendDeadlines()
	etag, err := rq.g.cfg.Service.PutObject(rq.ctx, rq.workspaceID, b, rq.objectKey, body.Size, rq.g.cfg.Service.PaceUpload(rq.ctx, rq.workspaceID, body.Body), headers)
	if err != nil {
		if f := body.Failure(); f != nil {
			return f
		}
		return rq.putError(err)
	}
	rq.addCORS(b)
	echoChecksums(rq.w, body.Verified())
	rq.w.Header().Set("ETag", etag)
	rq.w.WriteHeader(http.StatusOK)
	return nil
}

// putError maps what a streamed put can fail with beyond the object store's usual
// errors.
func (rq *request) putError(err error) *Error {
	switch {
	case errors.Is(err, objectstorage.ErrTooManyStreams):
		return errSlowDown("Too many uploads and downloads are in flight for this workspace; retry shortly.")
	case errors.Is(err, objectstorage.ErrIncompleteBody):
		return newError(http.StatusBadRequest, "IncompleteBody", "You did not provide the number of bytes specified by the Content-Length HTTP header.")
	case errors.Is(err, objectstorage.ErrMissingContentLength):
		return newError(http.StatusLengthRequired, "MissingContentLength", "You must provide the Content-Length HTTP header.")
	}
	return rq.fail(err)
}

func (rq *request) getObject() *Error {
	b, e := rq.bucketRow()
	if e != nil {
		return e
	}
	opts := conditionsFrom(rq.r.Header, "")
	opts.Range = rq.r.Header.Get("Range")
	rd, open, err := rq.g.cfg.Service.OpenObjectWith(rq.ctx, rq.workspaceID, b, rq.objectKey, opts)
	switch {
	case errors.Is(err, objectstorage.ErrNotModified):
		rq.addCORS(b)
		rq.w.WriteHeader(http.StatusNotModified)
		return nil
	case errors.Is(err, objectstorage.ErrPreconditionFailed):
		return newError(http.StatusPreconditionFailed, "PreconditionFailed", "At least one of the pre-conditions you specified did not hold")
	case errors.Is(err, objectstorage.ErrRangeNotSatisfiable):
		return newError(http.StatusRequestedRangeNotSatisfiable, "InvalidRange", "The requested range is not satisfiable")
	case err != nil:
		return rq.putError(err)
	}
	defer rd.Close()
	// The download is audited once per request, when bex hands the bytes out: an open
	// object, never a missing one (ADR112 D6).
	rq.recordGet(b)
	rq.extendDeadlines()
	rq.objectHeaders(rd.Info)
	rq.addCORS(b)
	status := http.StatusOK
	length := rd.Info.Size
	if open.ContentRange != "" {
		status = http.StatusPartialContent
		rq.w.Header().Set("Content-Range", open.ContentRange)
	}
	rq.w.Header().Set("Content-Length", strconv.FormatInt(length, 10))
	rq.w.WriteHeader(status)
	// The status is out: a failure from here is a truncated body, which the client retries.
	_, _ = io.Copy(rq.g.cfg.Service.PaceDownload(rq.ctx, rq.workspaceID, rq.w), rd)
	return nil
}

// objectHeaders sets what describes the object on a get or a head: the stored entity
// headers, the validators, the user metadata and the headers that keep a browser from
// running a tenant's HTML on bex's origin (ADR095 D7): attachment unless the object
// stored a disposition, nosniff and a sandboxing CSP.
func (rq *request) objectHeaders(info objectstorage.ObjectInfo) {
	h := rq.w.Header()
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
	if info.Headers.ContentDisposition != "" {
		h.Set("Content-Disposition", info.Headers.ContentDisposition)
	} else {
		h.Set("Content-Disposition", "attachment")
	}
	if info.Headers.ContentType == "" {
		h.Set("Content-Type", "binary/octet-stream")
	}
	set("ETag", info.ETag)
	if !info.LastModified.IsZero() {
		h.Set("Last-Modified", info.LastModified.UTC().Format(http.TimeFormat))
	}
	h.Set("Accept-Ranges", "bytes")
	h.Set("X-Content-Type-Options", "nosniff")
	h.Set("Content-Security-Policy", "default-src 'none'; sandbox")
	for k, v := range info.Headers.Metadata {
		h.Set("x-amz-meta-"+k, v)
	}
}

func (rq *request) headObject() *Error {
	b, e := rq.bucketRow()
	if e != nil {
		return e
	}
	info, err := rq.g.cfg.Service.StatObject(rq.ctx, rq.workspaceID, b, rq.objectKey)
	if err != nil {
		return rq.fail(err)
	}
	rq.objectHeaders(info)
	rq.addCORS(b)
	switch conditionsFrom(rq.r.Header, "").ConditionStatus(info.ETag, info.LastModified) {
	case http.StatusPreconditionFailed:
		return newError(http.StatusPreconditionFailed, "PreconditionFailed", "At least one of the pre-conditions you specified did not hold")
	case http.StatusNotModified:
		rq.w.WriteHeader(http.StatusNotModified)
		return nil
	}
	rq.w.Header().Set("Content-Length", strconv.FormatInt(info.Size, 10))
	rq.w.WriteHeader(http.StatusOK)
	return nil
}

// conditionsFrom reads the four conditional headers a read sends, or, with prefix
// "X-Amz-Copy-Source-", the ones a copy sends about its source. A header that is not a
// valid HTTP date is ignored, as S3 does.
func conditionsFrom(h http.Header, prefix string) objectstorage.GetOptions {
	o := objectstorage.GetOptions{IfMatch: h.Get(prefix + "If-Match"), IfNoneMatch: h.Get(prefix + "If-None-Match")}
	o.IfModifiedSince, _ = httpTime(h.Get(prefix + "If-Modified-Since"))
	o.IfUnmodifiedSince, _ = httpTime(h.Get(prefix + "If-Unmodified-Since"))
	return o
}

func httpTime(v string) (time.Time, bool) {
	if v == "" {
		return time.Time{}, false
	}
	t, err := http.ParseTime(v)
	return t, err == nil
}

func (rq *request) deleteObject() *Error {
	b, e := rq.bucketRow()
	if e != nil {
		return e
	}
	// A key that is not there is deleted, as on S3 (Render's route answers 404, ADR112 D2).
	if err := rq.g.cfg.Service.RemoveObject(rq.ctx, rq.workspaceID, b, rq.objectKey); err != nil {
		return rq.fail(err)
	}
	rq.w.WriteHeader(http.StatusNoContent)
	return nil
}

type deleteXML struct {
	XMLName xml.Name `xml:"Delete"`
	Quiet   bool     `xml:"Quiet"`
	Objects []struct {
		Key       string `xml:"Key"`
		VersionID string `xml:"VersionId"`
	} `xml:"Object"`
}

type deleteResultXML struct {
	XMLName xml.Name         `xml:"DeleteResult"`
	Xmlns   string           `xml:"xmlns,attr"`
	Deleted []deletedXML     `xml:"Deleted"`
	Errors  []deleteErrorXML `xml:"Error"`
}

type deletedXML struct {
	Key string `xml:"Key"`
}

type deleteErrorXML struct {
	Key     string `xml:"Key"`
	Code    string `xml:"Code"`
	Message string `xml:"Message"`
}

func (rq *request) deleteObjects() *Error {
	b, e := rq.bucketRow()
	if e != nil {
		return e
	}
	body, e := rq.readSmall(maxDeleteBody)
	if e != nil {
		return e
	}
	var doc deleteXML
	if e := decodeXML(body, &doc); e != nil {
		return e
	}
	if len(doc.Objects) == 0 || len(doc.Objects) > objectstorage.ListMaxLimit {
		return errMalformedXML(fmt.Sprintf("A delete request names 1 to %d objects.", objectstorage.ListMaxLimit))
	}
	keys := make([]string, 0, len(doc.Objects))
	for _, o := range doc.Objects {
		if o.VersionID != "" {
			return errNotImplemented("Object versions are not implemented")
		}
		keys = append(keys, o.Key)
	}
	deleted, failed, err := rq.g.cfg.Service.DeleteObjects(rq.ctx, rq.workspaceID, b, keys)
	if err != nil {
		return rq.fail(err)
	}
	out := deleteResultXML{Xmlns: s3Namespace}
	if !doc.Quiet {
		for _, k := range deleted {
			out.Deleted = append(out.Deleted, deletedXML{Key: k})
		}
	}
	for _, f := range failed {
		code, msg := "InternalError", "We encountered an internal error. Please try again."
		if f.Invalid {
			code, msg = "InvalidArgument", "The object key is not valid."
		}
		out.Errors = append(out.Errors, deleteErrorXML{Key: f.Key, Code: code, Message: msg})
	}
	rq.recordDeleteObjects(b, len(keys), keys[0])
	return writeXML(rq.w, http.StatusOK, out)
}

type taggingXML struct {
	XMLName xml.Name `xml:"Tagging"`
	Xmlns   string   `xml:"xmlns,attr"`
	TagSet  struct{} `xml:"TagSet"`
}

// getObjectTagging answers an empty tag set: the AWS CLI asks for tags while it
// copies, and an error here breaks `mv` and `sync` (ADR112 D3). Tag writes are 501.
func (rq *request) getObjectTagging() *Error {
	b, e := rq.bucketRow()
	if e != nil {
		return e
	}
	if _, err := rq.g.cfg.Service.StatObject(rq.ctx, rq.workspaceID, b, rq.objectKey); err != nil {
		return rq.fail(err)
	}
	return writeXML(rq.w, http.StatusOK, taggingXML{Xmlns: s3Namespace})
}

type copyResultXML struct {
	XMLName      xml.Name `xml:"CopyObjectResult"`
	Xmlns        string   `xml:"xmlns,attr"`
	LastModified string   `xml:"LastModified"`
	ETag         string   `xml:"ETag"`
}

// parseCopySource reads x-amz-copy-source: "<bucket>/<key>", percent-encoded, with an
// optional leading slash. A version id is 501.
func parseCopySource(raw string) (bucket, key string, e *Error) {
	if strings.Contains(raw, "?versionId=") {
		return "", "", errNotImplemented("Object versions are not implemented")
	}
	decoded, err := url.PathUnescape(raw)
	if err != nil {
		return "", "", errInvalidArgument("x-amz-copy-source is not valid")
	}
	bucket, key, _ = strings.Cut(strings.TrimPrefix(decoded, "/"), "/")
	if bucket == "" || key == "" {
		return "", "", errInvalidArgument("x-amz-copy-source must be <bucket>/<key>")
	}
	return bucket, key, nil
}

// copySource is the source of a copy or a part copy: x-amz-copy-source names a bex bucket of
// the key's workspace and a valid key, and reading it needs the creator's can_view_sensitive
// whatever the destination needs.
func (rq *request) copySource() (store.ObjectBucket, string, *Error) {
	srcName, srcKey, e := parseCopySource(rq.r.Header.Get(copySourceHeader))
	if e != nil {
		return store.ObjectBucket{}, "", e
	}
	if err := objectstorage.ValidateKey(srcKey); err != nil {
		return store.ObjectBucket{}, "", errInvalidArgument("The copy source key is not valid.")
	}
	src, err := rq.g.cfg.Service.ResolveBucket(rq.ctx, rq.workspaceID, srcName)
	if err != nil {
		return store.ObjectBucket{}, "", rq.fail(err)
	}
	if e := rq.authorize(core.RelCanViewSensitive, objectstorage.AuditVerbCopyObject, core.ObjectTarget(src.Name, srcKey)); e != nil {
		return store.ObjectBucket{}, "", e
	}
	return src, srcKey, nil
}

// copyObject copies inside the key's workspace and region (ADR112 D3). It needs the
// read of the source, which is the creator's can_view_sensitive, and the write of the
// destination, which the route already authorized; across regions or workspaces it is
// 501. It never reads or writes a byte through bex.
func (rq *request) copyObject() *Error {
	dst, e := rq.bucketRow()
	if e != nil {
		return e
	}
	src, srcKey, e := rq.copySource()
	if e != nil {
		return e
	}
	var replace *objectstorage.ObjectHeaders
	switch strings.ToUpper(rq.r.Header.Get("X-Amz-Metadata-Directive")) {
	case "", "COPY":
		if src.ID == dst.ID && srcKey == rq.objectKey {
			return errInvalidRequest("This copy request is illegal because it is trying to copy an object to itself without changing the object's metadata, storage class, website redirect location or encryption attributes.")
		}
	case "REPLACE":
		h, e := rq.entityHeaders()
		if e != nil {
			return e
		}
		replace = &h
	default:
		return errInvalidArgument("Unknown metadata directive.")
	}
	if rq.r.ContentLength > 0 {
		return errInvalidArgument("A copy request has no body.")
	}
	rq.extendDeadlines()
	res, err := rq.g.cfg.Service.CopyObject(rq.ctx, rq.workspaceID, src, srcKey, dst, rq.objectKey, replace, conditionsFrom(rq.r.Header, copySourceHeaderPrefix))
	if err != nil {
		return rq.putError(err)
	}
	rq.g.cfg.Base.RecordAllowedWrite(rq.ctx, objectstorage.AuditVerbCopyObject, core.RelCanCreate, rq.workspaceID,
		core.ObjectTarget(dst.Name, rq.objectKey), "copied from "+core.ObjectTarget(src.Name, srcKey))
	rq.addCORS(dst)
	return writeXML(rq.w, http.StatusOK, copyResultXML{Xmlns: s3Namespace, LastModified: res.LastModified.UTC().Format(s3Time), ETag: res.ETag})
}
