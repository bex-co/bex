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
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/bex-co/bex/lego/backend/internal/s3client"
)

// ErrObjectNotFound is a key that is not in the bucket.
var ErrObjectNotFound = errors.New("object not found")

// ErrNoSuchUpload is a multipart upload that Wasabi does not hold, or that is not the
// workspace's, bucket's or key's.
var ErrNoSuchUpload = errors.New("no such multipart upload")

// CompletedPart is one part of a CompleteMultipart: its number and the ETag its upload
// answered with.
type CompletedPart struct {
	Number int
	ETag   string
}

// ObjectInfo is what a HEAD or a listing says about one object.
type ObjectInfo struct {
	Key          string
	Size         int64
	ETag         string
	LastModified time.Time
	// Headers are the entity headers the object was stored with; Head and
	// GetObject fill them, a listing does not.
	Headers ObjectHeaders
}

// ObjectHeaders are the entity headers an object keeps, stored as the writer
// sent them (ADR112 D3). Expires is the header's text, not a parsed time, so
// what a writer sent is what a reader gets back.
type ObjectHeaders struct {
	ContentType        string
	CacheControl       string
	ContentDisposition string
	ContentEncoding    string
	ContentLanguage    string
	Expires            string
	// Metadata is the object's user metadata, the x-amz-meta-* headers without the
	// prefix and in lower case, as S3 returns them. rclone keeps an object's
	// modification time in x-amz-meta-mtime, and without it every sync copies
	// everything again (ADR112 D3).
	Metadata map[string]string
}

// ObjectBody is an object being read: its metadata and a stream of its bytes,
// which the caller closes.
type ObjectBody struct {
	ObjectInfo
	Body io.ReadCloser
	// ContentRange is the Content-Range of a partial answer ("bytes 0-9/100"),
	// empty when the whole object is being read.
	ContentRange string
}

// GetOptions are the range and conditions of a read, passed to the store as the
// caller sent them (ADR112 D3).
type GetOptions struct {
	Range                              string
	IfMatch, IfNoneMatch               string
	IfModifiedSince, IfUnmodifiedSince time.Time
}

// What a read can answer other than an object.
var (
	// ErrNotModified is a read whose If-None-Match or If-Modified-Since matched.
	ErrNotModified = errors.New("not modified")
	// ErrPreconditionFailed is a read whose If-Match or If-Unmodified-Since did not.
	ErrPreconditionFailed = errors.New("precondition failed")
	// ErrRangeNotSatisfiable is a Range the object cannot satisfy.
	ErrRangeNotSatisfiable = errors.New("range not satisfiable")
)

// CopyResult is what a server-side copy produced.
type CopyResult struct {
	ETag         string
	LastModified time.Time
}

// ListPage is one page of a prefix listing. Next is the key to pass as
// startAfter for the page after it, empty on the last page.
type ListPage struct {
	Objects []ObjectInfo
	Next    string
}

// DelimitedPage is one page of a listing with a delimiter.
type DelimitedPage struct {
	Objects        []ObjectInfo
	CommonPrefixes []string
	Truncated      bool
	// Last is the greatest key or common prefix on the page: where the next page starts
	// after, once the caller has turned a common prefix into the first key past it.
	Last string
}

// Store is one region's shared Wasabi bucket. Every key it takes is a full
// Wasabi key, <workspace id>/<bucket id>/<key>, built by ObjectKey.
type Store interface {
	// PresignPut returns a PUT URL that signs Content-Length = size, so an
	// upload cannot exceed what the quota gate reserved.
	PresignPut(ctx context.Context, key string, size int64, ttl time.Duration) (string, error)
	// PresignGet returns a GET URL that makes a browser download the object
	// instead of rendering tenant content on Wasabi's origin.
	PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error)
	// Head returns the object's metadata, or ErrObjectNotFound.
	Head(ctx context.Context, key string) (ObjectInfo, error)
	// Delete removes one object; a missing key is not an error (S3's answer).
	Delete(ctx context.Context, key string) error
	// PutStream stores size bytes read from body at key, overwriting. It sends
	// the real Content-Length, never a chunked body: Wasabi refuses one
	// (ADR112 § Not verified, settled 2026-11-15). Nothing is buffered or
	// written to disk. It returns the stored object's ETag. A body that ends
	// before size bytes stores nothing.
	PutStream(ctx context.Context, key string, size int64, body io.Reader, headers ObjectHeaders) (etag string, err error)
	// GetObject opens key for reading, or returns ErrObjectNotFound. Its headers
	// are the stored ones, and the body is never decompressed.
	GetObject(ctx context.Context, key string) (ObjectBody, error)
	// GetObjectWith opens key with a range and conditions. A condition that
	// holds back the object is ErrNotModified or ErrPreconditionFailed, and a
	// range past its end ErrRangeNotSatisfiable (Wasabi answers a presigned one
	// with the whole object, which is detected from the answer, ADR112 § Not
	// verified).
	GetObjectWith(ctx context.Context, key string, opts GetOptions) (ObjectBody, error)
	// Copy copies one object to another key of the same bucket inside Wasabi. A
	// nil headers keeps the source's entity headers and metadata (the COPY
	// directive); otherwise they are replaced (REPLACE).
	Copy(ctx context.Context, srcKey, dstKey string, headers *ObjectHeaders) (CopyResult, error)
	// List returns up to limit objects under prefix whose keys sort after
	// startAfter.
	List(ctx context.Context, prefix, startAfter string, limit int) (ListPage, error)
	// ListDelimited is List with a delimiter: keys whose name after prefix holds the
	// delimiter are folded into the common prefix that ends at its first occurrence.
	// Truncated reports whether more entries follow, and Last is the key or common
	// prefix the page ended on (the last of either, in key order).
	ListDelimited(ctx context.Context, prefix, delimiter, startAfter string, limit int) (DelimitedPage, error)
	// CreateMultipart starts a multipart upload of key with the object's headers and
	// returns Wasabi's upload id.
	CreateMultipart(ctx context.Context, key string, headers ObjectHeaders) (uploadID string, err error)
	// UploadPartStream streams size bytes from body as part number of the upload through a
	// presigned UploadPart URL that signs the length, and returns the part's ETag.
	UploadPartStream(ctx context.Context, key, uploadID string, part int, size int64, body io.Reader) (etag string, err error)
	// UploadPartCopy copies srcKey, or the byte range of it ("bytes=a-b", "" for all),
	// into a part of the upload.
	UploadPartCopy(ctx context.Context, srcKey, dstKey, uploadID string, part int, byteRange string) (CopyResult, error)
	// CompleteMultipart assembles the upload from the listed parts and returns the
	// object's ETag. ErrNoSuchUpload is an upload Wasabi no longer holds.
	CompleteMultipart(ctx context.Context, key, uploadID string, parts []CompletedPart) (etag string, err error)
	// AbortMultipart discards an upload and its parts; one Wasabi no longer holds is
	// ErrNoSuchUpload.
	AbortMultipart(ctx context.Context, key, uploadID string) error
	// DeleteKeys removes up to ListMaxLimit keys in one request (S3's
	// multi-object delete), which lifecycle rules need to finish inside the
	// hour. It returns the keys it could not delete and an error when there were
	// any; a key that was already gone is deleted.
	DeleteKeys(ctx context.Context, keys []string) (failed []string, err error)
	// DeletePrefix removes every object under prefix and reports how many it
	// removed. A failure part-way leaves the rest for a retry.
	DeletePrefix(ctx context.Context, prefix string) (int, error)
}

// Config is one region's connection. Production fills it from the region's
// constants and the credential (Region.Config); tests point Endpoint at a fake.
type Config struct {
	Endpoint, Region, Bucket string
	AccessKey, SecretKey     string
	// MaxAttempts caps the SDK's retries; zero keeps its default. Tests set 1
	// so an unavailable fake fails at once.
	MaxAttempts int
}

// Config returns the region's Wasabi connection with the given credential.
func (r Region) Config(accessKey, secretKey string) Config {
	return Config{Endpoint: r.Endpoint(), Region: r.WasabiRegion(), Bucket: r.Bucket(), AccessKey: accessKey, SecretKey: secretKey}
}

// putClient carries PutStream's bytes. It has no overall timeout (a 300 MiB
// body takes as long as the caller's link does; the request context bounds it),
// keeps as many idle connections to Wasabi as bex-api allows streams so a burst
// of small uploads does not pay a handshake each, and waits at most two minutes
// for Wasabi to answer once it has the whole body.
var putClient = &http.Client{Transport: func() *http.Transport {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.MaxIdleConnsPerHost = MaxConcurrentStreamsPerReplica
	t.ResponseHeaderTimeout = 2 * time.Minute
	return t
}()}

type wasabiStore struct {
	client    *s3.Client
	presigner *s3.PresignClient
	bucket    string
}

// NewStore builds the S3 client of one region's bucket.
func NewStore(cfg Config) Store {
	client := s3client.New(s3client.Config{
		Endpoint: cfg.Endpoint, Region: cfg.Region, AccessKey: cfg.AccessKey, SecretKey: cfg.SecretKey,
		MaxAttempts: cfg.MaxAttempts,
	})
	return &wasabiStore{client: client, presigner: s3.NewPresignClient(client), bucket: cfg.Bucket}
}

func (w *wasabiStore) PresignPut(ctx context.Context, key string, size int64, ttl time.Duration) (string, error) {
	out, err := w.presigner.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(w.bucket), Key: aws.String(key), ContentLength: aws.Int64(size),
	}, func(p *s3.PresignOptions) { p.Expires = ttl })
	if err != nil {
		return "", err
	}
	return out.URL, nil
}

func (w *wasabiStore) PresignGet(ctx context.Context, key string, ttl time.Duration) (string, error) {
	out, err := w.presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(w.bucket), Key: aws.String(key),
		ResponseContentDisposition: aws.String("attachment"),
	}, func(p *s3.PresignOptions) { p.Expires = ttl })
	if err != nil {
		return "", err
	}
	return out.URL, nil
}

func (w *wasabiStore) Head(ctx context.Context, key string) (ObjectInfo, error) {
	out, err := w.client.HeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(w.bucket), Key: aws.String(key)})
	if err != nil {
		if isNotFound(err) {
			return ObjectInfo{}, ErrObjectNotFound
		}
		return ObjectInfo{}, err
	}
	return ObjectInfo{
		Key: key, Size: aws.ToInt64(out.ContentLength), ETag: aws.ToString(out.ETag), LastModified: aws.ToTime(out.LastModified),
		Headers: ObjectHeaders{
			ContentType: aws.ToString(out.ContentType), CacheControl: aws.ToString(out.CacheControl),
			ContentDisposition: aws.ToString(out.ContentDisposition), ContentEncoding: aws.ToString(out.ContentEncoding),
			ContentLanguage: aws.ToString(out.ContentLanguage), Expires: expiresText(out.ExpiresString, out.Expires),
			Metadata: out.Metadata,
		},
	}, nil
}

// expiresText is an object's Expires header as the writer sent it, or, when the
// SDK could only parse it, as an HTTP date.
func expiresText(raw *string, parsed *time.Time) string {
	if s := aws.ToString(raw); s != "" {
		return s
	}
	if parsed != nil {
		return parsed.UTC().Format(http.TimeFormat)
	}
	return ""
}

func (w *wasabiStore) PutStream(ctx context.Context, key string, size int64, body io.Reader, h ObjectHeaders) (string, error) {
	if size < 0 {
		return "", fmt.Errorf("put stream: negative size %d", size)
	}
	// The presigned URL signs the length and the user metadata, so what is stored
	// is what the caller sent and no more; the entity headers travel unsigned.
	signed, err := w.presigner.PresignPutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(w.bucket), Key: aws.String(key), ContentLength: aws.Int64(size), Metadata: h.Metadata,
	}, func(p *s3.PresignOptions) { p.Expires = PresignTTL })
	if err != nil {
		return "", fmt.Errorf("presign put: %w", err)
	}
	return putSigned(ctx, signed, size, body, &h, "put "+key)
}

// putSigned sends size bytes of body to a presigned URL that signs the length. A zero
// length needs http.NoBody: net/http sends any other body with an unknown length
// chunked, which Wasabi refuses. The entity headers h carries (nil for none) travel
// unsigned.
func putSigned(ctx context.Context, signed *v4.PresignedHTTPRequest, size int64, body io.Reader, h *ObjectHeaders, what string) (string, error) {
	if size == 0 {
		body = http.NoBody
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, signed.URL, body)
	if err != nil {
		return "", err
	}
	req.ContentLength = size
	for name, values := range signed.SignedHeader {
		if strings.EqualFold(name, "host") || strings.EqualFold(name, "content-length") {
			continue
		}
		req.Header[http.CanonicalHeaderKey(name)] = values
	}
	if h != nil {
		for name, value := range map[string]string{
			"Content-Type": h.ContentType, "Cache-Control": h.CacheControl, "Content-Disposition": h.ContentDisposition,
			"Content-Encoding": h.ContentEncoding, "Content-Language": h.ContentLanguage, "Expires": h.Expires,
		} {
			if value != "" {
				req.Header.Set(name, value)
			}
		}
	}
	resp, err := putClient.Do(req)
	if err != nil {
		// net/http's error carries the whole URL, whose query is a signature
		// that stays valid for 15 minutes; keep it out of logs.
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			err = urlErr.Err
		}
		return "", fmt.Errorf("%s: %w", what, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		detail, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", fmt.Errorf("%s: wasabi answered %s: %s", what, resp.Status, strings.TrimSpace(string(detail)))
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	return resp.Header.Get("ETag"), nil
}

func (w *wasabiStore) CreateMultipart(ctx context.Context, key string, h ObjectHeaders) (string, error) {
	in := &s3.CreateMultipartUploadInput{
		Bucket: aws.String(w.bucket), Key: aws.String(key), Metadata: h.Metadata,
		ContentType: nonEmpty(h.ContentType), CacheControl: nonEmpty(h.CacheControl), ContentDisposition: nonEmpty(h.ContentDisposition),
		ContentEncoding: nonEmpty(h.ContentEncoding), ContentLanguage: nonEmpty(h.ContentLanguage),
	}
	if h.Expires != "" {
		if t, err := http.ParseTime(h.Expires); err == nil {
			in.Expires = aws.Time(t)
		}
	}
	out, err := w.client.CreateMultipartUpload(ctx, in)
	if err != nil {
		return "", err
	}
	return aws.ToString(out.UploadId), nil
}

func (w *wasabiStore) UploadPartStream(ctx context.Context, key, uploadID string, part int, size int64, body io.Reader) (string, error) {
	if size < 0 {
		return "", fmt.Errorf("upload part: negative size %d", size)
	}
	signed, err := w.presigner.PresignUploadPart(ctx, &s3.UploadPartInput{
		Bucket: aws.String(w.bucket), Key: aws.String(key), UploadId: aws.String(uploadID),
		PartNumber: aws.Int32(int32(part)), ContentLength: aws.Int64(size),
	}, func(p *s3.PresignOptions) { p.Expires = PresignTTL })
	if err != nil {
		return "", fmt.Errorf("presign upload part: %w", err)
	}
	etag, err := putSigned(ctx, signed, size, body, nil, fmt.Sprintf("upload part %d of %s", part, key))
	if err != nil && strings.Contains(err.Error(), "NoSuchUpload") {
		return "", ErrNoSuchUpload
	}
	return etag, err
}

func (w *wasabiStore) UploadPartCopy(ctx context.Context, srcKey, dstKey, uploadID string, part int, byteRange string) (CopyResult, error) {
	in := &s3.UploadPartCopyInput{
		Bucket: aws.String(w.bucket), Key: aws.String(dstKey), UploadId: aws.String(uploadID), PartNumber: aws.Int32(int32(part)),
		CopySource: aws.String(w.bucket + "/" + url.PathEscape(srcKey)), CopySourceRange: nonEmpty(byteRange),
	}
	out, err := w.client.UploadPartCopy(ctx, in)
	switch {
	case isNoSuchUpload(err):
		return CopyResult{}, ErrNoSuchUpload
	case isNotFound(err):
		return CopyResult{}, ErrObjectNotFound
	case err != nil:
		return CopyResult{}, err
	}
	res := CopyResult{}
	if out.CopyPartResult != nil {
		res.ETag, res.LastModified = aws.ToString(out.CopyPartResult.ETag), aws.ToTime(out.CopyPartResult.LastModified)
	}
	return res, nil
}

func (w *wasabiStore) CompleteMultipart(ctx context.Context, key, uploadID string, parts []CompletedPart) (string, error) {
	completed := make([]types.CompletedPart, 0, len(parts))
	for _, p := range parts {
		completed = append(completed, types.CompletedPart{PartNumber: aws.Int32(int32(p.Number)), ETag: aws.String(p.ETag)})
	}
	out, err := w.client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket: aws.String(w.bucket), Key: aws.String(key), UploadId: aws.String(uploadID),
		MultipartUpload: &types.CompletedMultipartUpload{Parts: completed},
	})
	switch {
	case isNoSuchUpload(err):
		return "", ErrNoSuchUpload
	case err != nil:
		return "", err
	}
	return aws.ToString(out.ETag), nil
}

func (w *wasabiStore) AbortMultipart(ctx context.Context, key, uploadID string) error {
	_, err := w.client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
		Bucket: aws.String(w.bucket), Key: aws.String(key), UploadId: aws.String(uploadID),
	})
	if isNoSuchUpload(err) {
		return ErrNoSuchUpload
	}
	return err
}

func (w *wasabiStore) GetObject(ctx context.Context, key string) (ObjectBody, error) {
	return w.GetObjectWith(ctx, key, GetOptions{})
}

func (w *wasabiStore) GetObjectWith(ctx context.Context, key string, opts GetOptions) (ObjectBody, error) {
	in := &s3.GetObjectInput{Bucket: aws.String(w.bucket), Key: aws.String(key)}
	if opts.Range != "" {
		in.Range = aws.String(opts.Range)
	}
	if opts.IfMatch != "" {
		in.IfMatch = aws.String(opts.IfMatch)
	}
	if opts.IfNoneMatch != "" {
		in.IfNoneMatch = aws.String(opts.IfNoneMatch)
	}
	if !opts.IfModifiedSince.IsZero() {
		in.IfModifiedSince = aws.Time(opts.IfModifiedSince)
	}
	if !opts.IfUnmodifiedSince.IsZero() {
		in.IfUnmodifiedSince = aws.Time(opts.IfUnmodifiedSince)
	}
	out, err := w.client.GetObject(ctx, in)
	if err != nil {
		var re *awshttp.ResponseError
		if errors.As(err, &re) {
			switch re.HTTPStatusCode() {
			case http.StatusNotModified:
				return ObjectBody{}, ErrNotModified
			case http.StatusPreconditionFailed:
				return ObjectBody{}, ErrPreconditionFailed
			case http.StatusRequestedRangeNotSatisfiable:
				return ObjectBody{}, ErrRangeNotSatisfiable
			}
		}
		if isNotFound(err) {
			return ObjectBody{}, ErrObjectNotFound
		}
		return ObjectBody{}, err
	}
	body := ObjectBody{
		ObjectInfo: ObjectInfo{
			Key: key, Size: aws.ToInt64(out.ContentLength), ETag: aws.ToString(out.ETag), LastModified: aws.ToTime(out.LastModified),
			Headers: ObjectHeaders{
				ContentType: aws.ToString(out.ContentType), CacheControl: aws.ToString(out.CacheControl),
				ContentDisposition: aws.ToString(out.ContentDisposition), ContentEncoding: aws.ToString(out.ContentEncoding),
				ContentLanguage: aws.ToString(out.ContentLanguage), Expires: expiresText(out.ExpiresString, out.Expires),
				Metadata: out.Metadata,
			},
		},
		Body: out.Body, ContentRange: aws.ToString(out.ContentRange),
	}
	return body, nil
}

func (w *wasabiStore) Copy(ctx context.Context, srcKey, dstKey string, h *ObjectHeaders) (CopyResult, error) {
	in := &s3.CopyObjectInput{
		Bucket: aws.String(w.bucket), Key: aws.String(dstKey),
		CopySource: aws.String(w.bucket + "/" + url.PathEscape(srcKey)),
	}
	if h != nil {
		in.MetadataDirective = types.MetadataDirectiveReplace
		in.Metadata = h.Metadata
		in.ContentType, in.CacheControl = nonEmpty(h.ContentType), nonEmpty(h.CacheControl)
		in.ContentDisposition, in.ContentEncoding, in.ContentLanguage = nonEmpty(h.ContentDisposition), nonEmpty(h.ContentEncoding), nonEmpty(h.ContentLanguage)
		if h.Expires != "" {
			if t, err := http.ParseTime(h.Expires); err == nil {
				in.Expires = aws.Time(t)
			}
		}
	}
	out, err := w.client.CopyObject(ctx, in)
	if err != nil {
		if isNotFound(err) {
			return CopyResult{}, ErrObjectNotFound
		}
		return CopyResult{}, err
	}
	res := CopyResult{}
	if out.CopyObjectResult != nil {
		res.ETag, res.LastModified = aws.ToString(out.CopyObjectResult.ETag), aws.ToTime(out.CopyObjectResult.LastModified)
	}
	return res, nil
}

func nonEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func (w *wasabiStore) Delete(ctx context.Context, key string) error {
	_, err := w.client.DeleteObject(ctx, &s3.DeleteObjectInput{Bucket: aws.String(w.bucket), Key: aws.String(key)})
	if err != nil && !isNotFound(err) {
		return err
	}
	return nil
}

func (w *wasabiStore) List(ctx context.Context, prefix, startAfter string, limit int) (ListPage, error) {
	if limit < 1 || limit > ListMaxLimit {
		return ListPage{}, fmt.Errorf("list limit %d is outside 1..%d", limit, ListMaxLimit)
	}
	in := &s3.ListObjectsV2Input{Bucket: aws.String(w.bucket), Prefix: aws.String(prefix), MaxKeys: aws.Int32(int32(limit))}
	if startAfter != "" {
		in.StartAfter = aws.String(startAfter)
	}
	out, err := w.client.ListObjectsV2(ctx, in)
	if err != nil {
		return ListPage{}, err
	}
	page := ListPage{Objects: make([]ObjectInfo, 0, len(out.Contents))}
	for _, o := range out.Contents {
		page.Objects = append(page.Objects, ObjectInfo{
			Key: aws.ToString(o.Key), Size: aws.ToInt64(o.Size), ETag: aws.ToString(o.ETag), LastModified: aws.ToTime(o.LastModified),
		})
	}
	if aws.ToBool(out.IsTruncated) && len(page.Objects) > 0 {
		page.Next = page.Objects[len(page.Objects)-1].Key
	}
	return page, nil
}

func (w *wasabiStore) ListDelimited(ctx context.Context, prefix, delimiter, startAfter string, limit int) (DelimitedPage, error) {
	if limit < 0 || limit > ListMaxLimit {
		return DelimitedPage{}, fmt.Errorf("list limit %d is outside 0..%d", limit, ListMaxLimit)
	}
	if limit == 0 {
		return DelimitedPage{}, nil
	}
	in := &s3.ListObjectsV2Input{Bucket: aws.String(w.bucket), Prefix: aws.String(prefix), MaxKeys: aws.Int32(int32(limit))}
	if delimiter != "" {
		in.Delimiter = aws.String(delimiter)
	}
	if startAfter != "" {
		in.StartAfter = aws.String(startAfter)
	}
	out, err := w.client.ListObjectsV2(ctx, in)
	if err != nil {
		return DelimitedPage{}, err
	}
	page := DelimitedPage{Truncated: aws.ToBool(out.IsTruncated), Objects: make([]ObjectInfo, 0, len(out.Contents))}
	for _, o := range out.Contents {
		key := aws.ToString(o.Key)
		page.Objects = append(page.Objects, ObjectInfo{Key: key, Size: aws.ToInt64(o.Size), ETag: aws.ToString(o.ETag), LastModified: aws.ToTime(o.LastModified)})
		page.Last = max(page.Last, key)
	}
	for _, p := range out.CommonPrefixes {
		pre := aws.ToString(p.Prefix)
		page.CommonPrefixes = append(page.CommonPrefixes, pre)
		page.Last = max(page.Last, pre)
	}
	return page, nil
}

func (w *wasabiStore) DeletePrefix(ctx context.Context, prefix string) (int, error) {
	removed := 0
	for {
		page, err := w.List(ctx, prefix, "", ListMaxLimit)
		if err != nil {
			return removed, err
		}
		if len(page.Objects) == 0 {
			return removed, nil
		}
		keys := make([]string, len(page.Objects))
		for i, o := range page.Objects {
			keys[i] = o.Key
		}
		failed, err := w.DeleteKeys(ctx, keys)
		removed += len(keys) - len(failed)
		if err != nil {
			return removed, fmt.Errorf("under %q: %w", prefix, err)
		}
	}
}

// isNotFound recognises S3's missing-key and missing-object answers, which a
// HEAD reports with no body as a bare 404.
// isNoSuchUpload reports a multipart upload Wasabi does not hold.
func isNoSuchUpload(err error) bool {
	var nsu *types.NoSuchUpload
	if errors.As(err, &nsu) {
		return true
	}
	var apiErr interface{ ErrorCode() string }
	return errors.As(err, &apiErr) && apiErr.ErrorCode() == "NoSuchUpload"
}

func isNotFound(err error) bool {
	var nf *types.NotFound
	if errors.As(err, &nf) {
		return true
	}
	var apiErr interface{ ErrorCode() string } // smithy.APIError, without importing it
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NotFound", "NoSuchKey":
			return true
		}
	}
	return false
}
