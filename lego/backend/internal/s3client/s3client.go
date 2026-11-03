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

// Package s3client builds the S3 client every bex-api object-store reader and
// writer uses: static credentials against an S3-compatible endpoint (Wasabi in
// production) with path-style addressing.
package s3client

import (
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// Config names one S3-compatible store and the credential to use there.
type Config struct {
	Endpoint, Region, AccessKey, SecretKey, SessionToken string
	// ChecksumsWhenRequired limits request checksums to the operations that
	// require one, for a writer that streams bodies it cannot hash up front.
	ChecksumsWhenRequired bool
}

// New returns the client. An empty Region is us-east-1, which every
// S3-compatible store bex targets accepts for signing.
func New(cfg Config) *s3.Client {
	region := cfg.Region
	if region == "" {
		region = "us-east-1"
	}
	awsConfig := aws.Config{
		Region:      region,
		Credentials: aws.NewCredentialsCache(credentials.NewStaticCredentialsProvider(cfg.AccessKey, cfg.SecretKey, cfg.SessionToken)),
	}
	if cfg.ChecksumsWhenRequired {
		awsConfig.RequestChecksumCalculation = aws.RequestChecksumCalculationWhenRequired
	}
	return s3.NewFromConfig(awsConfig, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(cfg.Endpoint)
		// The bucket is a path segment, not a hostname label: Wasabi and Hetzner
		// need it, and bucket names with dots break virtual-host TLS anyway.
		o.UsePathStyle = true
	})
}
