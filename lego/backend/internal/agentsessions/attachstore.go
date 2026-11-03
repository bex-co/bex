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

package agentsessions

import (
	"context"
	"io"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"

	"github.com/bex-co/bex/lego/backend/internal/s3client"
)

// AttachmentObjects is the bex-agent-attachments bucket (ADR095 D1). bex-api is
// its only writer; keys are `<workspace>/<att-id>`.
type AttachmentObjects interface {
	// Put stores exactly size bytes from body. An S3 PUT is atomic, so a body
	// that fails or ends early leaves no object.
	Put(ctx context.Context, key string, body io.Reader, size int64, mediaType string) error
	Get(ctx context.Context, key string) (io.ReadCloser, error)
	// DeleteMany deletes the keys and returns those now gone; a key that was
	// already absent counts as gone.
	DeleteMany(ctx context.Context, keys []string) ([]string, error)
	// PresignGet returns a GET URL on the storage origin whose response carries
	// the given content type and disposition.
	PresignGet(ctx context.Context, key, mediaType, disposition string, ttl time.Duration) (string, error)
}

// S3AttachmentConfig is the BEX_AGENT_ATTACHMENTS_S3_* contract.
type S3AttachmentConfig struct {
	Endpoint, Bucket, Region, AccessKey, SecretKey string
}

// Configured reports whether any setting is present; a partial set is a
// startup error (cmd/api config), not a silently disabled feature.
func (c S3AttachmentConfig) Configured() bool {
	return c.Endpoint+c.Bucket+c.Region+c.AccessKey+c.SecretKey != ""
}

// Missing names the required settings a configured store lacks.
func (c S3AttachmentConfig) Missing() []string {
	var missing []string
	for name, value := range map[string]string{
		"BEX_AGENT_ATTACHMENTS_S3_ENDPOINT": c.Endpoint, "BEX_AGENT_ATTACHMENTS_S3_BUCKET": c.Bucket,
		"BEX_AGENT_ATTACHMENTS_S3_ACCESS_KEY": c.AccessKey, "BEX_AGENT_ATTACHMENTS_S3_SECRET_KEY": c.SecretKey,
	} {
		if strings.TrimSpace(value) == "" {
			missing = append(missing, name)
		}
	}
	return missing
}

type s3AttachmentObjects struct {
	client    *s3.Client
	presigner *s3.PresignClient
	bucket    string
}

// NewS3AttachmentObjects returns nil when the bucket is not configured; every
// attachment verb then answers AGENT_ATTACHMENTS_UNAVAILABLE.
func NewS3AttachmentObjects(cfg S3AttachmentConfig) AttachmentObjects {
	if !cfg.Configured() || len(cfg.Missing()) > 0 {
		return nil
	}
	client := s3client.New(s3client.Config{
		Endpoint: cfg.Endpoint, Region: cfg.Region, AccessKey: cfg.AccessKey, SecretKey: cfg.SecretKey,
		// A streamed upload cannot be hashed up front; it goes unsigned.
		ChecksumsWhenRequired: true,
	})
	return &s3AttachmentObjects{client: client, presigner: s3.NewPresignClient(client), bucket: cfg.Bucket}
}

func (o *s3AttachmentObjects) Put(ctx context.Context, key string, body io.Reader, size int64, mediaType string) error {
	_, err := o.client.PutObject(ctx, &s3.PutObjectInput{
		Bucket: aws.String(o.bucket), Key: aws.String(key),
		Body: body, ContentLength: aws.Int64(size), ContentType: aws.String(mediaType),
	}, s3.WithAPIOptions(v4.SwapComputePayloadSHA256ForUnsignedPayloadMiddleware))
	return err
}

func (o *s3AttachmentObjects) Get(ctx context.Context, key string) (io.ReadCloser, error) {
	out, err := o.client.GetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(o.bucket), Key: aws.String(key)})
	if err != nil {
		return nil, err
	}
	return out.Body, nil
}

func (o *s3AttachmentObjects) DeleteMany(ctx context.Context, keys []string) ([]string, error) {
	if len(keys) == 0 {
		return nil, nil
	}
	objects := make([]types.ObjectIdentifier, len(keys))
	for i, key := range keys {
		objects[i] = types.ObjectIdentifier{Key: aws.String(key)}
	}
	out, err := o.client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
		Bucket: aws.String(o.bucket), Delete: &types.Delete{Objects: objects, Quiet: aws.Bool(true)},
	})
	if err != nil {
		return nil, err
	}
	failed := map[string]bool{}
	for _, e := range out.Errors {
		failed[aws.ToString(e.Key)] = true
	}
	gone := make([]string, 0, len(keys))
	for _, key := range keys {
		if !failed[key] {
			gone = append(gone, key)
		}
	}
	return gone, nil
}

func (o *s3AttachmentObjects) PresignGet(ctx context.Context, key, mediaType, disposition string, ttl time.Duration) (string, error) {
	out, err := o.presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(o.bucket), Key: aws.String(key),
		ResponseContentType:        aws.String(mediaType),
		ResponseContentDisposition: aws.String(disposition),
	}, func(p *s3.PresignOptions) { p.Expires = ttl })
	if err != nil {
		return "", err
	}
	return out.URL, nil
}
