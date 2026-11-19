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
	"errors"
	"regexp"
	"strings"
)

// SystemBucketPrefix is reserved: a workspace's render-<region> bucket is
// where Render's /v1/objects routes read and write, so no user bucket may
// start with it.
const SystemBucketPrefix = "render-"

var (
	// ErrInvalidBucketName refuses a name that breaks the bucket-name rule.
	ErrInvalidBucketName = errors.New("invalid bucket name")
	// ErrReservedBucketName refuses a name in the reserved render- namespace.
	ErrReservedBucketName = errors.New("bucket names starting with render- are reserved")
)

// bucketNameRE is the bucket-name rule: 3 to 63 characters, lower-case letters,
// digits and hyphens, starting and ending with a letter or digit (the DNS-label
// subset of S3's rule).
var bucketNameRE = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,61}[a-z0-9]$`)

// SystemBucketName is the name of a workspace's system bucket in region r.
func SystemBucketName(r Region) string { return SystemBucketPrefix + string(r) }

// IsSystemBucketName reports whether name is in the reserved namespace.
func IsSystemBucketName(name string) bool { return strings.HasPrefix(name, SystemBucketPrefix) }

// ValidateBucketName checks a name a user asked for. Names never change after
// creation.
func ValidateBucketName(name string) error {
	if !bucketNameRE.MatchString(name) {
		return ErrInvalidBucketName
	}
	if IsSystemBucketName(name) {
		return ErrReservedBucketName
	}
	return nil
}
