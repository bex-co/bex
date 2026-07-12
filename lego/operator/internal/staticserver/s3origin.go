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
	body, err := drain(out.Body)
	if err != nil {
		return Object{}, fmt.Errorf("staticserver: read %q: %w", key, err)
	}
	ct := ""
	if out.ContentType != nil {
		ct = *out.ContentType
	}
	return Object{Body: body, ContentType: ct}, nil
}

// isNotFound recognizes a missing-key error across the typed errors and the
// generic API-error codes different S3-compatibles return.
func isNotFound(err error) bool {
	var nsk *s3types.NoSuchKey
	var nf *s3types.NotFound
	if errors.As(err, &nsk) || errors.As(err, &nf) {
		return true
	}
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "NoSuchKey", "NotFound", "404":
			return true
		}
	}
	return false
}
