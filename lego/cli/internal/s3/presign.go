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
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	sdk "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/spf13/cobra"
)

// maxPresign is the longest a presigned URL may last, seven days, as on S3.
const maxPresign = 7 * 24 * time.Hour

func presignCommand(d deps, g *globals) *cobra.Command {
	var expiresIn int
	c := &cobra.Command{
		Use:   "presign s3://bucket/key",
		Short: "Print a URL that reads an object for a while, with no key",
		Long: "Prints a URL that lets anyone who has it download the object until it expires,\n" +
			"--expires-in seconds from now (3600 by default, at most 604800, seven days).\n\n" +
			"A URL dies with the key that signed it. bex makes a key for you that lasts twelve\n" +
			"hours, so a URL you ask to last longer stops working when the key does; the\n" +
			"command says so when that is the case. For a URL that lasts, sign it with a\n" +
			"persistent key (`bex s3 keys create`, then BEX_S3_ACCESS_KEY_ID and\n" +
			"BEX_S3_SECRET_ACCESS_KEY).",
		Example: "  bex s3 presign s3://photos/cat.jpg\n  bex s3 presign s3://photos/cat.jpg --expires-in 300",
		Args:    cobra.ExactArgs(1),
		RunE: func(c *cobra.Command, args []string) error {
			loc, err := parseRemote(args[0])
			if err != nil {
				return err
			}
			if loc.Key == "" {
				return usageError("%q names a bucket: presign takes an object, as s3://%s/key", args[0], loc.Bucket)
			}
			life := time.Duration(expiresIn) * time.Second
			if life < time.Second || life > maxPresign {
				return usageError("--expires-in must be between 1 and %d seconds (seven days), not %d", int(maxPresign/time.Second), expiresIn)
			}
			r, err := d.open(c, g, defaultConcurrency, true)
			if err != nil {
				return err
			}
			defer r.close()
			signed, err := sdk.NewPresignClient(r.client, func(o *sdk.PresignOptions) { o.Expires = life }).
				PresignGetObject(r.ctx, &sdk.GetObjectInput{Bucket: aws.String(loc.Bucket), Key: aws.String(loc.Key)})
			if err != nil {
				return r.fail(err)
			}
			now := d.now()
			if !r.sess.ExpiresAt.IsZero() && now.Add(life).After(r.sess.ExpiresAt) {
				_, _ = fmt.Fprintf(r.errOut, "note: this URL stops working at %s, when the key that signed it expires, not after %s. "+
					"A URL that lasts needs a persistent key (`bex s3 keys create`).\n", r.sess.ExpiresAt.UTC().Format(time.RFC3339), life)
				life = r.sess.ExpiresAt.Sub(now)
			}
			if r.format != formatText {
				return writeValue(r.out, r.format, map[string]any{
					"url": signed.URL, "expiresIn": int64(life / time.Second), "expiresAt": now.Add(life).UTC().Format(time.RFC3339),
				})
			}
			_, _ = fmt.Fprintln(r.out, signed.URL)
			return nil
		},
	}
	c.Flags().IntVar(&expiresIn, "expires-in", 3600, "seconds the URL works for, at most 604800")
	return c
}
