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
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/bex-co/bex/lego/backend/internal/s3client"
)

// ErrObjectNotFound is a key that is not in the bucket.
var ErrObjectNotFound = errors.New("object not found")

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
}

// ObjectBody is an object being read: its metadata and a stream of its bytes,
// which the caller closes.
type ObjectBody struct {
	ObjectInfo
	Body io.ReadCloser
}

// ListPage is one page of a prefix listing. Next is the key to pass as
// startAfter for the page after it, empty on the last page.
type ListPage struct {
	Objects []ObjectInfo
	Next    string
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
	// List returns up to limit objects under prefix whose keys sort after
	// startAfter.
	List(ctx context.Context, prefix, startAfter string, limit int) (ListPage, error)
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
	signed, err := w.PresignPut(ctx, key, size, PresignTTL)
	if err != nil {
		return "", fmt.Errorf("presign put: %w", err)
	}
	// A zero length needs http.NoBody: net/http sends any other body with an
	// unknown length chunked, which Wasabi refuses.
	if size == 0 {
		body = http.NoBody
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, signed, body)
	if err != nil {
		return "", err
	}
	req.ContentLength = size
	// Entity headers travel unsigned: the presigned URL signs only the length,
	// so the writer's own headers are stored as sent.
	for name, value := range map[string]string{
		"Content-Type": h.ContentType, "Cache-Control": h.CacheControl, "Content-Disposition": h.ContentDisposition,
		"Content-Encoding": h.ContentEncoding, "Content-Language": h.ContentLanguage, "Expires": h.Expires,
	} {
		if value != "" {
			req.Header.Set(name, value)
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
		return "", fmt.Errorf("put %s: %w", key, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode/100 != 2 {
		detail, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return "", fmt.Errorf("put %s: wasabi answered %s: %s", key, resp.Status, strings.TrimSpace(string(detail)))
	}
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	return resp.Header.Get("ETag"), nil
}

func (w *wasabiStore) GetObject(ctx context.Context, key string) (ObjectBody, error) {
	out, err := w.client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(w.bucket), Key: aws.String(key)})
	if err != nil {
		if isNotFound(err) {
			return ObjectBody{}, ErrObjectNotFound
		}
		return ObjectBody{}, err
	}
	return ObjectBody{
		ObjectInfo: ObjectInfo{
			Key: key, Size: aws.ToInt64(out.ContentLength), ETag: aws.ToString(out.ETag), LastModified: aws.ToTime(out.LastModified),
			Headers: ObjectHeaders{
				ContentType: aws.ToString(out.ContentType), CacheControl: aws.ToString(out.CacheControl),
				ContentDisposition: aws.ToString(out.ContentDisposition), ContentEncoding: aws.ToString(out.ContentEncoding),
				ContentLanguage: aws.ToString(out.ContentLanguage), Expires: expiresText(out.ExpiresString, out.Expires),
			},
		},
		Body: out.Body,
	}, nil
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
