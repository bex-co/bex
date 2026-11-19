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
	"net/http"
	"strconv"

	"github.com/bex-co/bex/lego/backend/internal/objectstorage"
)

// ListObjectsV2 and ListObjects (ADR112 D3): stateless paging over the object store, with
// a delimiter that folds keys into common prefixes and a continuation that never repeats
// or skips one.

func init() {
	handlers["ListObjectsV2"] = func(rq *request) *Error { return rq.listObjects(true) }
	handlers["ListObjects"] = func(rq *request) *Error { return rq.listObjects(false) }
}

type listBucketResultXML struct {
	XMLName               xml.Name          `xml:"ListBucketResult"`
	Xmlns                 string            `xml:"xmlns,attr"`
	Name                  string            `xml:"Name"`
	Prefix                string            `xml:"Prefix"`
	Delimiter             string            `xml:"Delimiter,omitempty"`
	MaxKeys               int               `xml:"MaxKeys"`
	EncodingType          string            `xml:"EncodingType,omitempty"`
	KeyCount              *int              `xml:"KeyCount,omitempty"`
	IsTruncated           bool              `xml:"IsTruncated"`
	ContinuationToken     string            `xml:"ContinuationToken,omitempty"`
	NextContinuationToken string            `xml:"NextContinuationToken,omitempty"`
	StartAfter            string            `xml:"StartAfter,omitempty"`
	Marker                *string           `xml:"Marker"`
	NextMarker            string            `xml:"NextMarker,omitempty"`
	Contents              []contentsXML     `xml:"Contents"`
	CommonPrefixes        []commonPrefixXML `xml:"CommonPrefixes"`
}

type contentsXML struct {
	Key          string `xml:"Key"`
	LastModified string `xml:"LastModified"`
	ETag         string `xml:"ETag"`
	Size         int64  `xml:"Size"`
	StorageClass string `xml:"StorageClass"`
}

type commonPrefixXML struct {
	Prefix string `xml:"Prefix"`
}

func (rq *request) listObjects(v2 bool) *Error {
	// A refused listing costs nothing: the limit comes before the bucket's row is read.
	if !rq.g.limits.allowListing(rq.workspaceID) {
		return errSlowDown("Too many listings from this workspace; Wasabi counts what a listing reads, so they are capped. Retry shortly.")
	}
	b, e := rq.bucketRow()
	if e != nil {
		return e
	}
	q := rq.q
	maxKeys := objectstorage.ListMaxLimit
	if v := q.Get("max-keys"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			return errInvalidArgument("max-keys must be a non-negative integer.")
		}
		maxKeys = n
	}
	encode := false
	switch q.Get("encoding-type") {
	case "":
	case "url":
		encode = true
	default:
		return errInvalidArgument("Invalid encoding type specified; the only one is url.")
	}
	req := objectstorage.ListRequest{
		V2: v2, Prefix: q.Get("prefix"), Delimiter: q.Get("delimiter"), MaxKeys: maxKeys,
		Token: q.Get("continuation-token"), StartAfter: q.Get("start-after"), Marker: q.Get("marker"),
	}
	res, err := rq.g.cfg.Service.ListS3(rq.ctx, rq.workspaceID, b, req)
	switch {
	case errors.Is(err, objectstorage.ErrDelimiterUnsupported):
		return errNotImplemented("Only a delimiter of one ASCII character is implemented.")
	case errors.Is(err, objectstorage.ErrInvalidCursor):
		return errInvalidArgument("The continuation token or marker provided is incorrect.")
	case err != nil:
		return rq.fail(err)
	}
	name := func(s string) string {
		if encode {
			return uriEncode(s, false)
		}
		return s
	}
	out := listBucketResultXML{
		Xmlns: s3Namespace, Name: b.Name, Prefix: name(req.Prefix), Delimiter: name(req.Delimiter), MaxKeys: maxKeys, IsTruncated: res.Truncated,
	}
	if encode {
		out.EncodingType = "url"
	}
	for _, o := range res.Objects {
		out.Contents = append(out.Contents, contentsXML{Key: name(o.Key), LastModified: o.LastModified.UTC().Format(s3Time), ETag: o.ETag, Size: o.SizeBytes, StorageClass: "STANDARD"})
	}
	for _, p := range res.CommonPrefixes {
		out.CommonPrefixes = append(out.CommonPrefixes, commonPrefixXML{Prefix: name(p)})
	}
	if v2 {
		count := len(out.Contents) + len(out.CommonPrefixes)
		out.KeyCount = &count
		out.ContinuationToken, out.NextContinuationToken, out.StartAfter = req.Token, res.NextToken, name(req.StartAfter)
	} else {
		marker := name(req.Marker)
		out.Marker, out.NextMarker = &marker, name(res.NextMarker)
	}
	rq.addCORS(b)
	return writeXML(rq.w, http.StatusOK, out)
}
