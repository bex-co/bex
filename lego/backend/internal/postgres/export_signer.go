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

package postgres

import (
	"context"
	"fmt"
	"net/url"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

// S3ExportSigner reads the out-of-band object-store credentials from the
// Database namespace and signs one GET. It does not retain credentials or URLs.
type S3ExportSigner struct {
	client client.Client
}

// NewS3ExportSigner creates the production logical-export URL signer.
func NewS3ExportSigner(kubeClient client.Client) *S3ExportSigner {
	return &S3ExportSigner{client: kubeClient}
}

// Presign implements ExportURLSigner.
func (s *S3ExportSigner) Presign(
	ctx context.Context,
	db *appv1alpha1.Database,
	export appv1alpha1.DatabaseExportStatus,
	ttl time.Duration,
) (string, error) {
	if s == nil || s.client == nil {
		return "", fmt.Errorf("kubernetes client is unavailable")
	}
	if db.Status.BackupEndpointURL == "" || db.Status.BackupS3SecretName == "" {
		return "", fmt.Errorf("database backup store coordinates are unavailable")
	}
	bucket, key, err := splitS3URL(export.ObjectKey)
	if err != nil {
		return "", err
	}
	if ttl <= 0 {
		return "", fmt.Errorf("download URL lifetime must be positive")
	}

	var secret corev1.Secret
	if err := s.client.Get(ctx, client.ObjectKey{Namespace: db.Namespace, Name: db.Status.BackupS3SecretName}, &secret); err != nil {
		return "", fmt.Errorf("read export credentials: %w", err)
	}
	accessKey := string(secret.Data["AWS_ACCESS_KEY_ID"])
	secretKey := string(secret.Data["AWS_SECRET_ACCESS_KEY"])
	if accessKey == "" || secretKey == "" {
		return "", fmt.Errorf("export credentials Secret must contain AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY")
	}
	region := firstNonEmpty(string(secret.Data["AWS_REGION"]), string(secret.Data["AWS_DEFAULT_REGION"]), "us-east-1")
	provider := credentials.NewStaticCredentialsProvider(accessKey, secretKey, string(secret.Data["AWS_SESSION_TOKEN"]))
	config := aws.Config{Region: region, Credentials: aws.NewCredentialsCache(provider)}
	objectClient := s3.NewFromConfig(config, func(options *s3.Options) {
		options.BaseEndpoint = aws.String(db.Status.BackupEndpointURL)
		options.UsePathStyle = true
	})
	presigner := s3.NewPresignClient(objectClient)
	result, err := presigner.PresignGetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(key),
		ResponseContentDisposition: aws.String(fmt.Sprintf(
			"attachment; filename=%q", export.Filename,
		)),
	}, func(options *s3.PresignOptions) {
		options.Expires = ttl
	})
	if err != nil {
		return "", err
	}
	return result.URL, nil
}

func splitS3URL(raw string) (string, string, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "s3" || u.Host == "" || strings.TrimPrefix(u.Path, "/") == "" {
		return "", "", fmt.Errorf("invalid export object key %q", raw)
	}
	return u.Host, strings.TrimPrefix(u.EscapedPath(), "/"), nil
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
