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
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/ratelimit"
	"github.com/aws/aws-sdk-go-v2/aws/retry"
	awshttp "github.com/aws/aws-sdk-go-v2/aws/transport/http"
	"github.com/aws/aws-sdk-go-v2/credentials"
	sdk "github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/smithy-go"
)

const (
	// defaultConcurrency and maxConcurrency bound the requests in flight at
	// once (`--concurrency`). The gateway admits 16 streams per workspace on
	// each replica (ADR112 D3), so eight leaves room for a second command in
	// the same workspace and ten is the most a person may ask for.
	defaultConcurrency = 8
	maxConcurrency     = 10
	// maxAttempts is how many times the SDK tries a request the gateway
	// answered with SlowDown or ServiceUnavailable, or that failed in transit.
	maxAttempts = 5
)

// newClient is the S3 client of one command. The configuration is built by hand
// and never loaded: nothing here reads a credential or configuration file, a
// profile or an AWS_ variable of the environment, so a person's own setup
// cannot change where a request goes or who signs it. The key is the one
// session resolved, the endpoint is the base endpoint, and bex serves path
// style only (ADR112 D3, D10). The SDK's own request checksums stay on: the
// gateway takes the checksum trailer (ADR112 D3, payload modes).
func (d deps) newClient(s session, concurrency int) *sdk.Client {
	var transport aws.HTTPClient = awshttp.NewBuildableClient()
	if d.httpClient != nil {
		transport = d.httpClient()
	}
	cfg := aws.Config{
		Region: s.Region,
		// AppID names bex in the User-Agent the gateway logs (and the contract suite reads).
		AppID:       "bex-cli",
		Credentials: credentials.NewStaticCredentialsProvider(s.AccessKeyID, s.Secret, ""),
		HTTPClient:  newLimitedClient(transport, concurrency),
		Retryer: func() aws.Retryer {
			return retry.NewStandard(func(o *retry.StandardOptions) {
				o.MaxAttempts = maxAttempts
				// A throttled request is retried however many were before it: the
				// default quota would stop a long transfer's retries early.
				o.RateLimiter = ratelimit.None
			})
		},
	}
	return sdk.NewFromConfig(cfg, func(o *sdk.Options) {
		o.BaseEndpoint = aws.String(s.Endpoint)
		o.UsePathStyle = true
	})
}

// limitedClient keeps at most `slots` requests in flight, whoever makes them:
// a request holds a slot from the moment it is sent until its response body is
// read or closed. `--concurrency` is therefore a bound on the whole command,
// files in flight times parts per file included, and a number the gateway's
// per-workspace stream cap can be set against.
type limitedClient struct {
	base  aws.HTTPClient
	slots chan struct{}
}

func newLimitedClient(base aws.HTTPClient, n int) *limitedClient {
	return &limitedClient{base: base, slots: make(chan struct{}, max(n, 1))}
}

func (l *limitedClient) Do(req *http.Request) (*http.Response, error) {
	select {
	case l.slots <- struct{}{}:
	case <-req.Context().Done():
		return nil, req.Context().Err()
	}
	resp, err := l.base.Do(req)
	if err != nil {
		<-l.slots
		return nil, err
	}
	resp.Body = &slotBody{ReadCloser: resp.Body, release: sync.OnceFunc(func() { <-l.slots })}
	return resp, nil
}

// slotBody frees its request's slot when the body ends or is closed.
type slotBody struct {
	io.ReadCloser
	release func()
}

func (b *slotBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	if err != nil {
		b.release()
	}
	return n, err
}

func (b *slotBody) Close() error {
	err := b.ReadCloser.Close()
	b.release()
	return err
}

// fault is an S3 error as the gateway reported it.
type fault struct {
	Code      string
	Message   string
	Status    int
	RequestID string
}

// faultOf reads the S3 error code and message out of an SDK error.
func faultOf(err error) (fault, bool) {
	var api smithy.APIError
	if !errors.As(err, &api) {
		return fault{}, false
	}
	f := fault{Code: api.ErrorCode(), Message: api.ErrorMessage()}
	var resp *awshttp.ResponseError
	if errors.As(err, &resp) {
		f.Status, f.RequestID = resp.HTTPStatusCode(), resp.ServiceRequestID()
	}
	return f, true
}

// keyGone is a refusal that means the access key itself is no good: it expired,
// was revoked, or never existed. A HEAD answer carries no body, so its 403 has
// only the status to go on.
func keyGone(f fault) bool {
	return f.Code == "InvalidAccessKeyId" || f.Code == "ExpiredToken" ||
		(f.Status == http.StatusForbidden && f.Code == "Forbidden")
}

// fatal is an error every later request of the command would meet too, so the
// command stops at the first instead of reporting it once per file.
func fatal(err error) bool {
	if errors.Is(err, context.Canceled) {
		return true
	}
	f, ok := faultOf(err)
	if !ok {
		return false
	}
	switch f.Code {
	case "InvalidAccessKeyId", "ExpiredToken", "SignatureDoesNotMatch", "RequestTimeTooSkewed", "QuotaExceeded", "AccessDenied", "NotImplemented":
		return true
	}
	return keyGone(f)
}

// persistentKeyHint is what a transfer that outlives its key needs.
const persistentKeyHint = "A transfer that runs longer than twelve hours needs a key that does not expire: " +
	"make one with `bex s3 keys create` and set BEX_S3_ACCESS_KEY_ID and BEX_S3_SECRET_ACCESS_KEY."

// explained is an error with bex's words that keeps the SDK's error behind it,
// so faultOf still reads the code.
type explained struct {
	msg string
	err error
}

func (e *explained) Error() string { return e.msg }
func (e *explained) Unwrap() error { return e.err }

// explain turns an SDK error into bex's message.
func explain(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) {
		return &explained{msg: "interrupted", err: err}
	}
	f, ok := faultOf(err)
	if !ok {
		// A request that never got an answer: say where it was going, not the
		// SDK's account of its attempts.
		var sent *url.Error
		if errors.As(err, &sent) {
			host := sent.URL
			if u, perr := url.Parse(sent.URL); perr == nil && u.Host != "" {
				host = u.Host
			}
			return &explained{msg: "could not reach " + host + ": " + sent.Err.Error(), err: err}
		}
		return err
	}
	detail := f.Code
	if msg := strings.TrimSpace(f.Message); msg != "" && msg != "UnknownError" {
		detail += ": " + msg
	}
	if f.RequestID != "" {
		detail += " (request " + f.RequestID + ")"
	}
	msg := detail
	switch {
	case keyGone(f):
		msg = "the S3 access key was refused: it has expired or been revoked, or its creator can no longer write here (" + detail + "). " +
			"Run the command again for a new key. " + persistentKeyHint
	case f.Code == "SignatureDoesNotMatch":
		msg = "the request signature was refused: check that the clock is right, and that BEX_S3_SECRET_ACCESS_KEY, if set, is the key's own secret (" + detail + ")"
	case f.Code == "RequestTimeTooSkewed":
		msg = "this computer's clock differs from the server's by more than the allowed skew: set it right and run the command again (" + detail + ")"
	case f.Code == "NotImplemented":
		msg = "bex object storage does not serve this operation (" + detail + ")"
	case f.Code == "SlowDown" || f.Code == "ServiceUnavailable":
		msg = "bex object storage is busy (" + detail + "); wait a moment, or run with a lower --concurrency"
	}
	return &explained{msg: msg, err: err}
}
