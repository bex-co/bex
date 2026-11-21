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

package s3

import (
	"context"
	"io"

	"github.com/aws/aws-sdk-go-v2/feature/s3/manager"
	sdk "github.com/aws/aws-sdk-go-v2/service/s3"
)

// The SDK's upload and download managers do the multipart work: splitting a
// body into parts, running them concurrently, retrying a part, and putting a
// download back together from ranged reads. This file is the only place they are
// used. The module is marked deprecated in favor of feature/s3/transfermanager,
// which is still a v0.x module without a compatibility guarantee (v0.5 at the
// time of writing); this client stays on the generally available one until the
// other is, and then only these two functions change (ADR112 D3, w21/m540).

// uploadParts sends one object: a single request when it fits in a part, else a
// multipart upload of partSize parts, --concurrency at a time.
//
//nolint:staticcheck // deprecated for transfermanager, which is not generally available; see above.
func (r *run) uploadParts(ctx context.Context, in *sdk.PutObjectInput, partSize int64) error {
	up := manager.NewUploader(r.client, func(u *manager.Uploader) {
		u.PartSize = partSize
		u.Concurrency = r.concurrency
	})
	_, err := up.Upload(ctx, in)
	return err
}

// downloadParts writes an object into w from ranged reads, --concurrency at a
// time, and returns the bytes written.
//
//nolint:staticcheck // deprecated for transfermanager, which is not generally available; see above.
func (r *run) downloadParts(ctx context.Context, w io.WriterAt, in *sdk.GetObjectInput) (int64, error) {
	dl := manager.NewDownloader(r.client, func(d *manager.Downloader) {
		d.PartSize = basePartSize
		d.Concurrency = r.concurrency
	})
	return dl.Download(ctx, w, in)
}
