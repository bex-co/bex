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
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
)

func (w *wasabiStore) DeleteKeys(ctx context.Context, keys []string) ([]string, error) {
	if len(keys) == 0 {
		return nil, nil
	}
	objects := make([]types.ObjectIdentifier, len(keys))
	for i, k := range keys {
		objects[i] = types.ObjectIdentifier{Key: aws.String(k)}
	}
	out, err := w.client.DeleteObjects(ctx, &s3.DeleteObjectsInput{
		Bucket: aws.String(w.bucket), Delete: &types.Delete{Objects: objects, Quiet: aws.Bool(true)},
	})
	if err != nil {
		return keys, err
	}
	if len(out.Errors) == 0 {
		return nil, nil
	}
	failed := make([]string, len(out.Errors))
	for i, e := range out.Errors {
		failed[i] = aws.ToString(e.Key)
	}
	return failed, fmt.Errorf("delete %d of %d objects: %s: %s",
		len(failed), len(keys), aws.ToString(out.Errors[0].Code), aws.ToString(out.Errors[0].Message))
}
