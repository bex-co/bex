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
	"net/http"
	"strconv"

	"github.com/bex-co/bex/lego/backend/internal/objectstorage"
)

// Multipart upload (ADR112 D3): what `aws s3 cp` of a file over 8 MB, `mv` and a
// bucket-to-bucket `sync` need. The upload id is bex's own; the rows and the Wasabi calls are
// the object store's.

func init() {
	handlers["CreateMultipartUpload"] = (*request).createMultipartUpload
	handlers["UploadPart"] = (*request).uploadPart
	handlers["CompleteMultipartUpload"] = (*request).completeMultipartUpload
	handlers["AbortMultipartUpload"] = (*request).abortMultipartUpload
	handlers["ListParts"] = (*request).listParts
	handlers["ListMultipartUploads"] = (*request).listMultipartUploads
}

// maxCompleteBody bounds a CompleteMultipartUpload document: 10,000 parts, each with an ETag
// and up to five checksums.
const maxCompleteBody = 4 << 20

type initiateMultipartXML struct {
	XMLName  xml.Name `xml:"InitiateMultipartUploadResult"`
	Xmlns    string   `xml:"xmlns,attr"`
	Bucket   string   `xml:"Bucket"`
	Key      string   `xml:"Key"`
	UploadID string   `xml:"UploadId"`
}

type completeMultipartRequestXML struct {
	Parts []struct {
		PartNumber int    `xml:"PartNumber"`
		ETag       string `xml:"ETag"`
	} `xml:"Part"`
}

type completeMultipartResultXML struct {
	XMLName  xml.Name `xml:"CompleteMultipartUploadResult"`
	Xmlns    string   `xml:"xmlns,attr"`
	Location string   `xml:"Location"`
	Bucket   string   `xml:"Bucket"`
	Key      string   `xml:"Key"`
	ETag     string   `xml:"ETag"`
}

type copyPartResultXML struct {
	XMLName      xml.Name `xml:"CopyPartResult"`
	Xmlns        string   `xml:"xmlns,attr"`
	LastModified string   `xml:"LastModified"`
	ETag         string   `xml:"ETag"`
}

type listPartsResultXML struct {
	XMLName              xml.Name  `xml:"ListPartsResult"`
	Xmlns                string    `xml:"xmlns,attr"`
	Bucket               string    `xml:"Bucket"`
	Key                  string    `xml:"Key"`
	UploadID             string    `xml:"UploadId"`
	StorageClass         string    `xml:"StorageClass"`
	PartNumberMarker     int       `xml:"PartNumberMarker"`
	NextPartNumberMarker int       `xml:"NextPartNumberMarker,omitempty"`
	MaxParts             int       `xml:"MaxParts"`
	IsTruncated          bool      `xml:"IsTruncated"`
	Parts                []partXML `xml:"Part"`
}

type partXML struct {
	PartNumber   int    `xml:"PartNumber"`
	LastModified string `xml:"LastModified"`
	ETag         string `xml:"ETag"`
	Size         int64  `xml:"Size"`
}

type listUploadsResultXML struct {
	XMLName            xml.Name    `xml:"ListMultipartUploadsResult"`
	Xmlns              string      `xml:"xmlns,attr"`
	Bucket             string      `xml:"Bucket"`
	KeyMarker          string      `xml:"KeyMarker"`
	UploadIDMarker     string      `xml:"UploadIdMarker"`
	NextKeyMarker      string      `xml:"NextKeyMarker,omitempty"`
	NextUploadIDMarker string      `xml:"NextUploadIdMarker,omitempty"`
	Prefix             string      `xml:"Prefix"`
	MaxUploads         int         `xml:"MaxUploads"`
	EncodingType       string      `xml:"EncodingType,omitempty"`
	IsTruncated        bool        `xml:"IsTruncated"`
	Uploads            []uploadXML `xml:"Upload"`
}

type uploadXML struct {
	Key          string `xml:"Key"`
	UploadID     string `xml:"UploadId"`
	StorageClass string `xml:"StorageClass"`
	Initiated    string `xml:"Initiated"`
}

func (rq *request) createMultipartUpload() *Error {
	b, e := rq.bucketRow()
	if e != nil {
		return e
	}
	headers, e := rq.entityHeaders()
	if e != nil {
		return e
	}
	id, err := rq.g.cfg.Service.CreateMultipartUpload(rq.ctx, rq.workspaceID, b, rq.objectKey, rq.key.ID, headers)
	if err != nil {
		return rq.fail(err)
	}
	rq.addCORS(b)
	return writeXML(rq.w, http.StatusOK, initiateMultipartXML{Xmlns: s3Namespace, Bucket: b.Name, Key: rq.objectKey, UploadID: id})
}

// partNumber reads partNumber, which S3 bounds to 1 through 10,000.
func (rq *request) partNumber() (int, *Error) {
	n, err := strconv.Atoi(rq.q.Get("partNumber"))
	if err != nil || n < 1 || n > objectstorage.MaxParts {
		return 0, errInvalidArgument("Part number must be an integer between 1 and 10000, inclusive.")
	}
	return n, nil
}

func (rq *request) uploadPart() *Error {
	if rq.r.Header.Get("X-Amz-Copy-Source") != "" {
		return rq.uploadPartCopy()
	}
	b, e := rq.bucketRow()
	if e != nil {
		return e
	}
	part, e := rq.partNumber()
	if e != nil {
		return e
	}
	body, e := rq.openBody(objectstorage.MaxPartBytes)
	if e != nil {
		return e
	}
	rq.extendDeadlines()
	etag, err := rq.g.cfg.Service.UploadPart(rq.ctx, rq.workspaceID, b, rq.objectKey, rq.q.Get("uploadId"), part, body.Size,
		rq.g.cfg.Service.PaceUpload(rq.ctx, rq.workspaceID, body.Body))
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

// uploadPartCopy copies a source object, or a range of it, into a part, inside Wasabi: the
// same source rules as CopyObject, and nothing moves through bex.
func (rq *request) uploadPartCopy() *Error {
	dst, e := rq.bucketRow()
	if e != nil {
		return e
	}
	part, e := rq.partNumber()
	if e != nil {
		return e
	}
	src, srcKey, e := rq.copySource()
	if e != nil {
		return e
	}
	rq.extendDeadlines()
	res, err := rq.g.cfg.Service.UploadPartCopy(rq.ctx, rq.workspaceID, src, srcKey, dst, rq.objectKey, rq.q.Get("uploadId"), part,
		objectstorage.PartCopyOptions{Range: rq.r.Header.Get("X-Amz-Copy-Source-Range"), Conditions: conditionsFrom(rq.r.Header, copySourceHeaderPrefix)})
	if err != nil {
		return rq.putError(err)
	}
	rq.addCORS(dst)
	return writeXML(rq.w, http.StatusOK, copyPartResultXML{Xmlns: s3Namespace, LastModified: res.LastModified.UTC().Format(s3Time), ETag: res.ETag})
}

func (rq *request) completeMultipartUpload() *Error {
	b, e := rq.bucketRow()
	if e != nil {
		return e
	}
	raw, e := rq.readSmall(maxCompleteBody)
	if e != nil {
		return e
	}
	var in completeMultipartRequestXML
	if e := decodeXML(raw, &in); e != nil {
		return e
	}
	parts := make([]objectstorage.CompletedPart, 0, len(in.Parts))
	for _, p := range in.Parts {
		parts = append(parts, objectstorage.CompletedPart{Number: p.PartNumber, ETag: p.ETag})
	}
	rq.extendDeadlines()
	etag, err := rq.g.cfg.Service.CompleteMultipartUpload(rq.ctx, rq.workspaceID, b, rq.objectKey, rq.q.Get("uploadId"), parts)
	if err != nil {
		return rq.putError(err)
	}
	rq.addCORS(b)
	return writeXML(rq.w, http.StatusOK, completeMultipartResultXML{
		Xmlns: s3Namespace, Location: "/" + b.Name + "/" + rq.objectKey, Bucket: b.Name, Key: rq.objectKey, ETag: etag,
	})
}

func (rq *request) abortMultipartUpload() *Error {
	b, e := rq.bucketRow()
	if e != nil {
		return e
	}
	if err := rq.g.cfg.Service.AbortMultipartUpload(rq.ctx, rq.workspaceID, b, rq.objectKey, rq.q.Get("uploadId")); err != nil {
		return rq.fail(err)
	}
	rq.addCORS(b)
	rq.w.WriteHeader(http.StatusNoContent)
	return nil
}

// listLimit reads a max-parts or max-uploads parameter: a missing one is the default of
// 1,000, and a negative or non-numeric one is InvalidArgument.
func (rq *request) listLimit(name string) (int, *Error) {
	v := rq.q.Get(name)
	if v == "" {
		return objectstorage.ListMaxLimit, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return 0, errInvalidArgument(name + " must be a non-negative integer.")
	}
	return n, nil
}

func (rq *request) listParts() *Error {
	b, e := rq.bucketRow()
	if e != nil {
		return e
	}
	limit, e := rq.listLimit("max-parts")
	if e != nil {
		return e
	}
	marker := 0
	if v := rq.q.Get("part-number-marker"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 0 {
			return errInvalidArgument("part-number-marker must be a non-negative integer.")
		}
		marker = n
	}
	page, err := rq.g.cfg.Service.ListParts(rq.ctx, rq.workspaceID, b, rq.objectKey, rq.q.Get("uploadId"), limit, marker)
	if err != nil {
		return rq.fail(err)
	}
	out := listPartsResultXML{
		Xmlns: s3Namespace, Bucket: b.Name, Key: rq.objectKey, UploadID: rq.q.Get("uploadId"), StorageClass: "STANDARD",
		PartNumberMarker: marker, NextPartNumberMarker: page.NextMarker, MaxParts: limit, IsTruncated: page.Truncated,
	}
	for _, p := range page.Parts {
		out.Parts = append(out.Parts, partXML{PartNumber: p.Number, LastModified: p.LastModified.Format(s3Time), ETag: p.ETag, Size: p.Size})
	}
	rq.addCORS(b)
	return writeXML(rq.w, http.StatusOK, out)
}

func (rq *request) listMultipartUploads() *Error {
	if rq.q.Get("delimiter") != "" {
		return errNotImplemented("A delimiter on ListMultipartUploads is not implemented.")
	}
	b, e := rq.bucketRow()
	if e != nil {
		return e
	}
	limit, e := rq.listLimit("max-uploads")
	if e != nil {
		return e
	}
	encode := false
	switch rq.q.Get("encoding-type") {
	case "":
	case "url":
		encode = true
	default:
		return errInvalidArgument("Invalid encoding type specified; the only one is url.")
	}
	name := func(s string) string {
		if encode {
			return uriEncode(s, false)
		}
		return s
	}
	keyMarker, idMarker := rq.q.Get("key-marker"), rq.q.Get("upload-id-marker")
	prefix := rq.q.Get("prefix")
	page, err := rq.g.cfg.Service.ListMultipartUploads(rq.ctx, rq.workspaceID, b, prefix, keyMarker, idMarker, limit)
	if err != nil {
		return rq.fail(err)
	}
	out := listUploadsResultXML{
		Xmlns: s3Namespace, Bucket: b.Name, KeyMarker: name(keyMarker), UploadIDMarker: idMarker, Prefix: name(prefix),
		NextKeyMarker: name(page.NextKeyMarker), NextUploadIDMarker: page.NextUploadIDMarker, MaxUploads: limit, IsTruncated: page.Truncated,
	}
	if encode {
		out.EncodingType = "url"
	}
	for _, u := range page.Uploads {
		out.Uploads = append(out.Uploads, uploadXML{Key: name(u.Key), UploadID: u.UploadID, StorageClass: "STANDARD", Initiated: u.Initiated.Format(s3Time)})
	}
	rq.addCORS(b)
	return writeXML(rq.w, http.StatusOK, out)
}
