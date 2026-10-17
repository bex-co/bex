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

package staticserver

import (
	"context"
	"errors"
	"fmt"
	"mime"
	"unicode/utf8"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	smithy "github.com/aws/smithy-go"
)

// S3Origin is an Origin backed by an S3-compatible object store (Wasabi/Hetzner
// today). It signs each GET, so the bucket stays private — the same credentials
// the publish Job uses to write (AWS_ACCESS_KEY_ID/AWS_SECRET_ACCESS_KEY from the
// environment). Path-style addressing is used because S3-compatibles rarely
// support virtual-host-style bucket subdomains.
type S3Origin struct {
	client *s3.Client
	bucket string
}

// Check verifies the configured identity can list the dedicated origin bucket.
// It is called once at startup so a missing, wrong, or over-rotated read Secret
// fails the static-server Deployment immediately instead of leaving a Ready pod
// that answers every tenant request with an opaque 503.
func (o *S3Origin) Check(ctx context.Context) error {
	maxKeys := int32(1)
	objects, err := o.client.ListObjectsV2(ctx, &s3.ListObjectsV2Input{
		Bucket:  aws.String(o.bucket),
		MaxKeys: &maxKeys,
	})
	if err != nil {
		return fmt.Errorf("staticserver: verify read access to bucket %q: %w", o.bucket, err)
	}
	// An empty new bucket has nothing to read yet. Once any published object is
	// present, a signed HEAD proves the identity also has GetObject rather than
	// merely enough ListBucket authority to appear healthy.
	if len(objects.Contents) > 0 && objects.Contents[0].Key != nil {
		if _, err := o.client.HeadObject(ctx, &s3.HeadObjectInput{
			Bucket: aws.String(o.bucket),
			Key:    objects.Contents[0].Key,
		}); err != nil {
			return fmt.Errorf("staticserver: verify object read access in bucket %q: %w", o.bucket, err)
		}
	}
	return nil
}

// NewS3Origin builds an S3-backed Origin. Credentials come from the standard AWS
// environment (the ServiceAccount-mounted Secret in-cluster). endpoint is the
// S3-compatible base URL (e.g. https://s3.eu-central-2.wasabisys.com); region
// defaults to us-east-1 when empty (required by the SDK even though path-style +
// a custom endpoint ignores it for routing).
func NewS3Origin(ctx context.Context, endpoint, region, bucket string) (*S3Origin, error) {
	if region == "" {
		region = "us-east-1"
	}
	cfg, err := awsconfig.LoadDefaultConfig(ctx, awsconfig.WithRegion(region))
	if err != nil {
		return nil, fmt.Errorf("staticserver: load aws config: %w", err)
	}
	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		if endpoint != "" {
			o.BaseEndpoint = aws.String(endpoint)
		}
		o.UsePathStyle = true
	})
	return &S3Origin{client: client, bucket: bucket}, nil
}

// Get fetches key from the bucket, mapping a missing object to ErrNotFound.
func (o *S3Origin) Get(ctx context.Context, key string) (Object, error) {
	out, err := o.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(o.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		if isNotFound(err) {
			return Object{}, ErrNotFound
		}
		return Object{}, fmt.Errorf("staticserver: get %q: %w", key, err)
	}
	// Reject up front when the origin reports a size beyond the bound, before
	// allocating; drain enforces the same cap defensively (codex-security #10).
	if out.ContentLength != nil && *out.ContentLength > maxOriginObjectBytes {
		_ = out.Body.Close()
		return Object{}, ErrObjectTooLarge
	}
	body, err := drain(out.Body)
	if err != nil {
		return Object{}, fmt.Errorf("staticserver: read %q: %w", key, err)
	}
	ct := aws.ToString(out.ContentType)
	// An encoded body's bytes are not its text, so they say nothing of its
	// charset.
	if aws.ToString(out.ContentEncoding) == "" {
		ct = utf8PlainText(ct, body)
	}
	return Object{Body: body, ContentType: ct}, nil
}

// utf8PlainText declares UTF-8 on a text/plain type that names no charset when
// the bytes are valid UTF-8. The publisher's uploader guesses a bare
// text/plain, which a browser decodes as windows-1252, so UTF-8 text showed as
// mojibake (w4/335). Judged once per fetch, before the object is cached: an
// explicit charset, invalid UTF-8, a malformed type and every other type stay
// as the origin sent them.
func utf8PlainText(contentType string, body []byte) string {
	mediaType, params, err := mime.ParseMediaType(contentType)
	if err != nil || mediaType != "text/plain" {
		return contentType
	}
	if _, declared := params["charset"]; declared || !utf8.Valid(body) {
		return contentType
	}
	params["charset"] = "utf-8"
	if utf8Type := mime.FormatMediaType(mediaType, params); utf8Type != "" {
		return utf8Type
	}
	return contentType
}

// isNotFound recognizes a missing-key error across the typed errors and the
// generic API-error codes different S3-compatibles return.
func isNotFound(err error) bool {
	var nsk *s3types.NoSuchKey
	var nf *s3types.NotFound
	if errors.As(err, &nsk) || errors.As(err, &nf) {
		return true
	}
	if apiErr, ok := errors.AsType[smithy.APIError](err); ok {
		switch apiErr.ErrorCode() {
		// KeyTooLongError is S3's answer to a key past its length cap — a key
		// that can never exist, so it maps to not-found like the rest. The
		// handler rejects such keys up front (w6/047); this covers a store
		// with a lower cap.
		case "NoSuchKey", "NotFound", "404", "KeyTooLongError":
			return true
		}
	}
	return false
}
