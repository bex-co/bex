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
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	sdk "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/spf13/cobra"
)

// deleteBatch is how many keys one DeleteObjects call carries, S3's maximum.
const deleteBatch = 1000

func rmCommand(d deps, g *globals) *cobra.Command {
	var recursive, dryrun bool
	var concurrency int
	var rules filters
	c := &cobra.Command{
		Use:   "rm s3://bucket/key",
		Short: "Delete an object, or everything under a prefix with --recursive",
		Long: "Deletes one object. With --recursive, deletes every object under the prefix, as\n" +
			"if it were a folder (`s3://photos/2023` means `s3://photos/2023/`), in batches of\n" +
			"1,000. --include and --exclude pick among them by a glob over the path below the\n" +
			"prefix, the last matching one deciding; --dryrun prints what would be deleted and\n" +
			"deletes nothing. A key that does not exist is not an error, as on S3.",
		Example: "  bex s3 rm s3://photos/cat.jpg\n  bex s3 rm s3://photos/tmp --recursive\n  bex s3 rm s3://photos/ --recursive --exclude '*' --include '*.tmp' --dryrun",
		Args:    cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			loc, err := parseRemote(args[0])
			if err != nil {
				return err
			}
			if !recursive && loc.Key == "" {
				return usageError("%q names a bucket: rm deletes an object (s3://%s/key), or with --recursive what is in it; `bex s3 rb` removes a bucket", args[0], loc.Bucket)
			}
			if !recursive && len(rules) > 0 {
				return usageError("--include and --exclude choose among the objects of a --recursive delete")
			}
			if err := checkConcurrency(concurrency); err != nil {
				return err
			}
			r, err := d.open(c, g, concurrency, true)
			if err != nil {
				return err
			}
			defer r.close()
			if !recursive {
				return r.deleteOne(loc, dryrun)
			}
			if err := r.deleteTree(loc.Bucket, asDir(loc.Key), rules, dryrun, concurrency); err != nil {
				return err
			}
			return r.rep.finish()
		},
	}
	c.Flags().BoolVar(&recursive, "recursive", false, "delete every object under the prefix")
	c.Flags().BoolVar(&dryrun, "dryrun", false, "show what would be deleted and delete nothing")
	c.Flags().Var(filterFlag{include: false, rules: &rules}, "exclude", "skip objects whose path below the prefix matches this glob (repeatable)")
	c.Flags().Var(filterFlag{include: true, rules: &rules}, "include", "delete objects whose path below the prefix matches this glob, after --exclude (repeatable)")
	concurrencyFlag(c, &concurrency)
	return c
}

func (r *run) deleteOne(loc location, dryrun bool) error {
	if !dryrun {
		if _, err := r.client.DeleteObject(r.ctx, &sdk.DeleteObjectInput{Bucket: aws.String(loc.Bucket), Key: aws.String(loc.Key)}); err != nil {
			return r.fail(err)
		}
	}
	r.rep.add(result{Action: "delete", Target: loc.String(), DryRun: dryrun})
	return r.rep.finish()
}

// deleteTree deletes every object of bucket under prefix that the filters let
// through, 1,000 keys to a call, with the calls running concurrently. It lists
// as it deletes, so a bucket of millions of keys is never held in memory.
func (r *run) deleteTree(bucket, prefix string, rules filters, dryrun bool, concurrency int) error {
	workers := r.newGroup(concurrency)
	var batch []types.ObjectIdentifier
	flush := func() {
		keys := batch
		batch = nil
		if len(keys) == 0 {
			return
		}
		workers.do(fmt.Sprintf("delete %d objects of s3://%s/%s", len(keys), bucket, prefix), func(ctx context.Context) error {
			return r.deleteKeys(ctx, bucket, keys)
		})
	}
	err := pageEach(workers.ctx, r.client, bucket, prefix, "", func(page *sdk.ListObjectsV2Output) error {
		for _, o := range page.Contents {
			key := aws.ToString(o.Key)
			if !rules.allows(strings.TrimPrefix(key, prefix)) {
				continue
			}
			if dryrun {
				r.rep.add(result{Action: "delete", Target: urlScheme + bucket + "/" + key, DryRun: true})
				continue
			}
			batch = append(batch, types.ObjectIdentifier{Key: aws.String(key)})
			if len(batch) == deleteBatch {
				flush()
			}
		}
		if !workers.alive() {
			return workers.ctx.Err()
		}
		return nil
	})
	flush()
	if waitErr := workers.wait(); waitErr != nil {
		return waitErr
	}
	return r.fail(err)
}

// deleteKeys deletes one batch and reports each key that went; a key the
// server refused is a failure line.
func (r *run) deleteKeys(ctx context.Context, bucket string, keys []types.ObjectIdentifier) error {
	out, err := r.client.DeleteObjects(ctx, &sdk.DeleteObjectsInput{
		Bucket: aws.String(bucket),
		Delete: &types.Delete{Objects: keys, Quiet: aws.Bool(true)},
	})
	if err != nil {
		return err
	}
	refused := map[string]bool{}
	var first error
	for _, e := range out.Errors {
		refused[aws.ToString(e.Key)] = true
		failure := fmt.Errorf("%s: %s", aws.ToString(e.Code), aws.ToString(e.Message))
		if first == nil {
			first = failure
		}
		r.prog.clear()
		_, _ = fmt.Fprintf(r.errOut, "failed: delete s3://%s/%s: %s\n", bucket, aws.ToString(e.Key), failure)
	}
	for _, k := range keys {
		if !refused[aws.ToString(k.Key)] {
			r.rep.add(result{Action: "delete", Target: urlScheme + bucket + "/" + aws.ToString(k.Key)})
		}
	}
	if first != nil {
		return reported{fmt.Errorf("%d of %d keys were not deleted, the first with %w", len(refused), len(keys), first)}
	}
	return nil
}
