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
	"encoding/json"
	"os"
	"slices"
	"strings"
	"testing"
)

// The Wasabi side of ADR112 is spelled out three times: the region constants in
// regions.go, the IAM policies under infra/wasabi, and the provisioning script.
// These tests fail when they drift, because a bucket the policy does not name
// is a bucket bex-api cannot reach, and a bucket it names that bex does not use
// is authority nothing needs.

const (
	rwPolicyPath     = "../../../../infra/wasabi/object-storage-s3-policy.json"
	roPolicyPath     = "../../../../infra/wasabi/object-storage-read-s3-policy.json"
	provisionerPath  = "../../../../scripts/object-storage-s3-credentials.sh"
	arnPrefix        = "arn:aws:s3:::"
	wantRWActionsObj = "s3:GetObject s3:PutObject s3:DeleteObject s3:AbortMultipartUpload s3:ListMultipartUploadParts"
	// wantRWActionsBucket are the bucket-level actions: the S3 surface lists a
	// bucket's open multipart uploads (ADR112 D3, D9).
	wantRWActionsBucket = "s3:GetBucketLocation s3:ListBucket s3:ListBucketMultipartUploads"
)

type policyDoc struct {
	Statement []struct {
		Action   []string `json:"Action"`
		Resource []string `json:"Resource"`
	} `json:"Statement"`
}

func readPolicy(t *testing.T, path string) policyDoc {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var doc policyDoc
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return doc
}

func wantResources() (buckets, objects []string) {
	for _, r := range Regions {
		buckets = append(buckets, arnPrefix+r.Bucket())
		objects = append(objects, arnPrefix+r.Bucket()+"/*")
	}
	slices.Sort(buckets)
	slices.Sort(objects)
	return buckets, objects
}

func TestPoliciesNameExactlyTheRegionBuckets(t *testing.T) {
	buckets, objects := wantResources()
	for _, path := range []string{rwPolicyPath, roPolicyPath} {
		doc := readPolicy(t, path)
		var gotBuckets, gotObjects []string
		for _, st := range doc.Statement {
			for _, res := range st.Resource {
				if strings.HasSuffix(res, "/*") {
					gotObjects = append(gotObjects, res)
				} else {
					gotBuckets = append(gotBuckets, res)
				}
			}
		}
		slices.Sort(gotBuckets)
		slices.Sort(gotObjects)
		if !slices.Equal(gotBuckets, buckets) || !slices.Equal(gotObjects, objects) {
			t.Errorf("%s names buckets %v and objects %v; regions.go says %v and %v", path, gotBuckets, gotObjects, buckets, objects)
		}
	}
}

func TestReadOnlyPolicyCannotWriteAndReadWritePolicyCan(t *testing.T) {
	actions := func(path string) []string {
		var all []string
		for _, st := range readPolicy(t, path).Statement {
			all = append(all, st.Action...)
		}
		return all
	}
	ro, rw := actions(roPolicyPath), actions(rwPolicyPath)
	for _, a := range ro {
		if strings.Contains(a, "Put") || strings.Contains(a, "Delete") || strings.Contains(a, "Abort") || a == "s3:*" || a == "*" {
			t.Errorf("objectgw serves the internet; its policy must not grant %s", a)
		}
	}
	for _, a := range strings.Fields(wantRWActionsObj + " " + wantRWActionsBucket) {
		if !slices.Contains(rw, a) {
			t.Errorf("bex-api's policy lacks %s", a)
		}
	}
	if !slices.Contains(ro, "s3:GetObject") || !slices.Contains(ro, "s3:ListBucket") {
		t.Errorf("objectgw's policy must allow get and list, has %v", ro)
	}
}

func TestProvisionerSpellsTheRegionsTheCodeServes(t *testing.T) {
	raw, err := os.ReadFile(provisionerPath)
	if err != nil {
		t.Fatal(err)
	}
	script := string(raw)
	for _, r := range Regions {
		spec := string(r) + ":" + r.WasabiRegion()
		if !strings.Contains(script, spec) {
			t.Errorf("scripts/object-storage-s3-credentials.sh does not list %q; regions.go serves it", spec)
		}
	}
	// The endpoint the script derives is the one the client dials.
	if !strings.Contains(script, `https://s3.%s.wasabisys.com`) {
		t.Error("the provisioner does not derive the S3 endpoint the way Region.Endpoint does")
	}
}
