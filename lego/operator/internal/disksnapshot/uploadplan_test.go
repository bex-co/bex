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

package disksnapshot

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

const (
	mib = int64(1) << 20
	gib = int64(1) << 30
	// diskMaxSizeGB is the largest disk the API accepts (backend/internal/apps
	// disks.go); a snapshot has to fit its parts for every size up to it.
	diskMaxSizeGB = 10000
	// s3MaxParts and s3MaxPartSize are S3's limits on one multipart upload.
	s3MaxParts    = 10000
	s3MaxPartSize = 5 * gib
)

// A snapshot is one stream whose length is unknown until its last byte. With the
// uploader's defaults (8 MiB parts, at most 10,000) a stream past about 78 GiB
// fails at the part cap and the disk never snapshots, so the part size has to
// follow the disk. The stream is the volume's contents plus tar, gzip and age
// framing, which on incompressible data is a little more than the volume.
func TestPlanUploadFitsEveryDiskTheAPIAllows(t *testing.T) {
	for _, sizeGB := range []int64{1, 10, 70, 78, 80, 100, 500, 1000, 1024, 5000, diskMaxSizeGB} {
		plan := PlanUpload(sizeGB)
		worstStream := sizeGB * gib * 11 / 10
		parts := (worstStream + plan.PartSize - 1) / plan.PartSize
		if parts > s3MaxParts {
			t.Errorf("%d GB: %d parts of %d MiB exceed S3's %d", sizeGB, parts, plan.PartSize/mib, s3MaxParts)
		}
		if plan.PartSize < 8*mib || plan.PartSize > s3MaxPartSize || plan.PartSize%mib != 0 {
			t.Errorf("%d GB: part size %d is not a whole number of MiB in [8 MiB, 5 GiB]", sizeGB, plan.PartSize)
		}
		if plan.Concurrency < 1 || plan.Concurrency > 5 {
			t.Errorf("%d GB: concurrency %d", sizeGB, plan.Concurrency)
		}
		// What the uploader holds in memory stays within a budget, unless one part alone exceeds it.
		if buffered := plan.BufferBytes(); buffered > max(256*mib, plan.PartSize) {
			t.Errorf("%d GB: %d MiB buffered", sizeGB, buffered/mib)
		}
	}
}

func TestPlanUploadKeepsTheDefaultsForDisksThatAlreadyWork(t *testing.T) {
	for _, sizeGB := range []int64{0, -1, 1, 10, 60} {
		if plan := PlanUpload(sizeGB); plan.PartSize != 8*mib || plan.Concurrency != 5 {
			t.Errorf("%d GB: plan = %+v, want the 8 MiB x 5 defaults", sizeGB, plan)
		}
	}
}

func TestPlanUploadPartsGrowWithTheDisk(t *testing.T) {
	small, large := PlanUpload(100), PlanUpload(10000)
	if small.PartSize >= large.PartSize {
		t.Fatalf("100 GB parts %d MiB, 10000 GB parts %d MiB", small.PartSize/mib, large.PartSize/mib)
	}
}

// Put has to hand the plan to the uploader: a 20 MiB stream in 5 MiB parts is
// four parts, where the 8 MiB default makes three. (The uploader sends a stream
// under 16 MiB as one object, so the stream is longer than that.)
func TestPutUploadsInThePlannedPartSize(t *testing.T) {
	for _, test := range []struct {
		name  string
		plan  UploadPlan
		parts int64
	}{
		{"default", UploadPlan{}, 3},
		{"5 MiB parts", UploadPlan{PartSize: 5 * mib, Concurrency: 1}, 4},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("AWS_ACCESS_KEY_ID", "test")
			t.Setenv("AWS_SECRET_ACCESS_KEY", "test-secret")
			t.Setenv("AWS_EC2_METADATA_DISABLED", "true")
			var uploaded atomic.Int64
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				query := r.URL.Query()
				w.Header().Set("Content-Type", "application/xml")
				switch {
				case r.Method == http.MethodPost && query.Has("uploads"):
					_, _ = fmt.Fprint(w, `<InitiateMultipartUploadResult><Bucket>b</Bucket><Key>k</Key><UploadId>u1</UploadId></InitiateMultipartUploadResult>`)
				case r.Method == http.MethodPut && query.Has("partNumber"):
					_, _ = bytes.NewBuffer(nil).ReadFrom(r.Body)
					uploaded.Add(1)
					w.Header().Set("ETag", `"etag"`)
				case r.Method == http.MethodPost && query.Has("uploadId"):
					_, _ = fmt.Fprint(w, `<CompleteMultipartUploadResult><Bucket>b</Bucket><Key>k</Key><ETag>"etag"</ETag></CompleteMultipartUploadResult>`)
				default:
					t.Errorf("unexpected %s %s", r.Method, r.URL)
				}
			}))
			defer server.Close()

			store := Store{Endpoint: server.URL, Bucket: "b", Upload: test.plan}
			// A pipe, like the real snapshot: its length is not known up front.
			stream := struct{ io.Reader }{bytes.NewReader(make([]byte, 20*mib))}
			if err := store.Put(t.Context(), "k", stream); err != nil {
				t.Fatal(err)
			}
			if got := uploaded.Load(); got != test.parts {
				t.Fatalf("parts uploaded = %d, want %d", got, test.parts)
			}
		})
	}
}
