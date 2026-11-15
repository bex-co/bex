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

package objectgw

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/service/s3"

	"github.com/bex-co/bex/lego/backend/internal/objectstorage"
	"github.com/bex-co/bex/lego/backend/internal/s3client"
)

// Origin fetches an object, or its headers, from the object store. The gateway
// hands the visitor's Range and conditional headers to Wasabi and relays what it
// answers, so partial content, 304 and 416 are Wasabi's own and the bytes are
// byte-identical to what was stored (ADR112 D4).
type Origin interface {
	// Fetch asks region's store for the object at the full Wasabi key, with GET
	// or HEAD, carrying the forwardable request headers. The caller closes the
	// response body.
	Fetch(ctx context.Context, region objectstorage.Region, key, method string, header http.Header) (*http.Response, error)
}

// presignTTL is how long the URL the gateway signs for one request is valid. It
// is used at once.
const presignTTL = time.Minute

// responseHeaderTimeout is how long Wasabi may take to start answering.
const responseHeaderTimeout = 30 * time.Second

// forwardedRequestHeaders are the visitor's headers that Wasabi evaluates. If-Range
// is not among them (S3 does not take it): see forwardHeaders.
var forwardedRequestHeaders = []string{"Range", "If-Match", "If-None-Match", "If-Modified-Since", "If-Unmodified-Since"}

// maxForwardedHeaderBytes bounds one forwarded header value.
const maxForwardedHeaderBytes = 512

// forwardHeaders picks the headers of the visitor's request that go to Wasabi.
// A request with If-Range is not partial here: the gateway cannot ask Wasabi to
// check the validator, and a ranged answer for an object that changed would
// corrupt a resumed download, so Range is dropped and the whole object served,
// which the standard allows a server to do.
func forwardHeaders(in http.Header) http.Header {
	out := http.Header{}
	for _, name := range forwardedRequestHeaders {
		if name == "Range" && in.Get("If-Range") != "" {
			continue
		}
		if v := in.Get(name); v != "" && len(v) <= maxForwardedHeaderBytes {
			out.Set(name, v)
		}
	}
	return out
}

type wasabiOrigin struct {
	presign map[objectstorage.Region]*s3.PresignClient
	bucket  map[objectstorage.Region]string
	client  *http.Client
}

// NewWasabiOrigin builds the origin over one read-only connection per region.
// It signs a short-lived GET or HEAD URL for each request and sends it with
// net/http, so the answer's status, headers and body are Wasabi's, not an SDK's
// rendering of them.
func NewWasabiOrigin(regions map[objectstorage.Region]objectstorage.Config) Origin {
	o := &wasabiOrigin{
		presign: map[objectstorage.Region]*s3.PresignClient{},
		bucket:  map[objectstorage.Region]string{},
		client: &http.Client{
			Transport: transport(),
			// A redirect from the store is a misconfiguration, not an answer to relay.
			CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
		},
	}
	for region, cfg := range regions {
		client := s3client.New(s3client.Config{
			Endpoint: cfg.Endpoint, Region: cfg.Region, AccessKey: cfg.AccessKey, SecretKey: cfg.SecretKey, MaxAttempts: 1,
		})
		o.presign[region] = s3.NewPresignClient(client)
		o.bucket[region] = cfg.Bucket
	}
	return o
}

// transport never decompresses: an object stored with Content-Encoding: gzip is
// served as stored, and a Content-Length is the stored length.
func transport() *http.Transport {
	t := http.DefaultTransport.(*http.Transport).Clone()
	t.DisableCompression = true
	// Room for every request a replica serves at once, so a burst does not
	// handshake again once it ends.
	t.MaxIdleConnsPerHost = MaxInFlight
	t.MaxIdleConns = 2 * MaxInFlight
	t.ResponseHeaderTimeout = responseHeaderTimeout
	return t
}

func (o *wasabiOrigin) Fetch(ctx context.Context, region objectstorage.Region, key, method string, header http.Header) (*http.Response, error) {
	presigner, ok := o.presign[region]
	if !ok {
		return nil, fmt.Errorf("no object store for region %q", region)
	}
	var (
		signed *v4.PresignedHTTPRequest
		err    error
	)
	expires := func(p *s3.PresignOptions) { p.Expires = presignTTL }
	switch method {
	case http.MethodGet:
		signed, err = presigner.PresignGetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(o.bucket[region]), Key: aws.String(key)}, expires)
	case http.MethodHead:
		signed, err = presigner.PresignHeadObject(ctx, &s3.HeadObjectInput{Bucket: aws.String(o.bucket[region]), Key: aws.String(key)}, expires)
	default:
		return nil, errors.New("only GET and HEAD are served")
	}
	if err != nil {
		return nil, fmt.Errorf("sign: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, method, signed.URL, nil)
	if err != nil {
		return nil, err
	}
	for name, values := range signed.SignedHeader {
		if http.CanonicalHeaderKey(name) != "Host" {
			req.Header[name] = values
		}
	}
	for name, values := range header {
		req.Header[name] = values
	}
	resp, err := o.client.Do(req)
	if err != nil {
		// net/http's error carries the whole URL, whose query is a signature
		// valid for a minute; keep it out of logs.
		var urlErr *url.Error
		if errors.As(err, &urlErr) {
			err = urlErr.Err
		}
		return nil, fmt.Errorf("object store: %w", err)
	}
	return resp, nil
}
