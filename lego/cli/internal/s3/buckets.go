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
	"errors"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	sdk "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/spf13/cobra"
)

// systemBucketPrefix is the reserved prefix of the buckets bex makes itself,
// Render's `ea objects` namespace among them (ADR112 D1, D7): no one can create
// one and none can be removed.
const systemBucketPrefix = "render-"

func mbCommand(d deps, g *globals) *cobra.Command {
	return &cobra.Command{
		Use:   "mb s3://bucket",
		Short: "Make a bucket (--region oregon|frankfurt)",
		Long: "Makes a bucket in the workspace, in the region --region names (oregon is the\n" +
			"default). A bucket name is 3 to 63 lowercase letters, digits and hyphens, unique\n" +
			"inside the workspace, and cannot start with `render-`.",
		Example: "  bex s3 mb s3://photos\n  bex s3 mb s3://photos-eu --region frankfurt",
		Args:    cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			loc, err := parseRemote(args[0])
			if err != nil {
				return err
			}
			if loc.Key != "" {
				return usageError("%q names an object: mb takes a bucket, as s3://%s", args[0], loc.Bucket)
			}
			r, err := d.open(c, g, defaultConcurrency, true)
			if err != nil {
				return err
			}
			defer r.close()
			_, err = r.client.CreateBucket(r.ctx, &sdk.CreateBucketInput{
				Bucket:                    aws.String(loc.Bucket),
				CreateBucketConfiguration: &types.CreateBucketConfiguration{LocationConstraint: types.BucketLocationConstraint(r.sess.Region)},
			})
			if err != nil {
				if f, ok := faultOf(err); ok && f.Code == "BucketAlreadyOwnedByYou" {
					return fmt.Errorf("the bucket %s already exists in this workspace (BucketAlreadyOwnedByYou)", loc.Bucket)
				}
				return r.fail(err)
			}
			r.rep.add(result{Action: "make_bucket", Target: loc.Bucket})
			return r.rep.finish()
		},
	}
}

func rbCommand(d deps, g *globals) *cobra.Command {
	var force, yes bool
	var concurrency int
	c := &cobra.Command{
		Use:   "rb s3://bucket",
		Short: "Remove a bucket; --force deletes its objects first",
		Long: "Removes a bucket. It must be empty: --force deletes every object in it first.\n" +
			"A bucket that still has unfinished multipart uploads cannot be removed either;\n" +
			"bex aborts them seven days after they start. A system bucket (`render-<region>`,\n" +
			"Render's `ea objects` namespace) cannot be removed.\n\n" +
			"--force asks \"Are you sure?\" in a terminal, as `bex ea objects delete` does;\n" +
			"--yes or --confirm skips the question, and a pipe, a script or CI is never asked.",
		Example: "  bex s3 rb s3://photos\n  bex s3 rb s3://photos --force",
		Args:    cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			loc, err := parseRemote(args[0])
			if err != nil {
				return err
			}
			if loc.Key != "" {
				return usageError("%q names an object: rb takes a bucket, as s3://%s", args[0], loc.Bucket)
			}
			if strings.HasPrefix(loc.Bucket, systemBucketPrefix) {
				return fmt.Errorf("%s is a system bucket (Render's `ea objects` namespace) and cannot be removed", loc.Bucket)
			}
			if err := checkConcurrency(concurrency); err != nil {
				return err
			}
			if force && !d.confirmBulkDelete(c, yes, "the bucket '"+loc.String()+"' and every object in it") {
				return nil
			}
			r, err := d.open(c, g, concurrency, true)
			if err != nil {
				return err
			}
			defer r.close()
			if force {
				if err := r.deleteTree(loc.Bucket, "", nil, false, concurrency); err != nil {
					return err
				}
			}
			if _, err := r.client.DeleteBucket(r.ctx, &sdk.DeleteBucketInput{Bucket: aws.String(loc.Bucket)}); err != nil {
				if f, ok := faultOf(err); ok && f.Code == "BucketNotEmpty" {
					return errors.New("the bucket " + loc.Bucket + " is not empty: --force deletes its objects first. " +
						"Unfinished multipart uploads also keep a bucket from being removed; bex aborts them seven days after they start (" + f.Message + ")")
				}
				return r.fail(err)
			}
			r.rep.add(result{Action: "remove_bucket", Target: loc.Bucket})
			return r.rep.finish()
		},
	}
	c.Flags().BoolVar(&force, "force", false, "delete every object in the bucket first")
	yesFlag(c, &yes)
	concurrencyFlag(c, &concurrency)
	return c
}
