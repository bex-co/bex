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

package api

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/graphql-go/graphql/gqlerrors"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// TestSanitizeGraphQLErrors proves the GraphQL surface shares core.WriteErr's
// redaction policy: an unclassified resolver error (raw pgx/k8s) is replaced
// with a generic message; classified/coded resolver errors and pure
// parse/validation errors keep their client-needed message. graphql-go wraps
// resolver errors in *gqlerrors.Error (no Unwrap), so the sanitizer must peel it.
func TestSanitizeGraphQLErrors(t *testing.T) {
	// graphql-go wraps a resolver error in *gqlerrors.Error, copying the
	// resolver's message into Message and keeping the error in OriginalError.
	wrap := func(resolverErr error) gqlerrors.FormattedError {
		return gqlerrors.FormatError(gqlerrors.NewError(resolverErr.Error(), nil, "", nil, nil, resolverErr))
	}

	leaky := errors.New(`pq: duplicate key violates constraint "x" host=10.0.0.5`)
	classified := fmt.Errorf("service %q: %w", "web", core.ErrNotFound)
	coded := core.NewPaymentRequiredError()
	// A pure parse/validation error wraps no resolver error (OriginalError nil).
	parseErr := gqlerrors.FormatError(gqlerrors.NewError("Syntax Error: unexpected }", nil, "", nil, nil, nil))

	out := sanitizeGraphQLErrors([]gqlerrors.FormattedError{
		wrap(leaky), wrap(classified), wrap(coded), parseErr,
	})

	if out[0].Message != "internal error" {
		t.Errorf("unclassified resolver error not redacted: %q", out[0].Message)
	}
	if out[1].Message == "internal error" {
		t.Errorf("classified (not found) resolver error wrongly redacted: %q", out[1].Message)
	}
	if out[2].Message == "internal error" || out[2].Message != core.PaymentRequiredMessage {
		t.Errorf("coded resolver error message wrong: %q", out[2].Message)
	}
	if out[3].Message != "Syntax Error: unexpected }" {
		t.Errorf("parse/validation error wrongly redacted: %q", out[3].Message)
	}
}

// TestGraphQLExecutionBudgetFitsInsideTheWriteDeadline pins w4/m140's fix: with
// the two equal, a resolver that ran to the deadline lost its connection before
// the error could be written, and the edge answered with a CORS-less 502.
func TestGraphQLExecutionBudgetFitsInsideTheWriteDeadline(t *testing.T) {
	if margin := HTTPWriteTimeout - gqlExecTimeout; margin < 3*time.Second {
		t.Fatalf("gqlExecTimeout %v leaves only %v under WriteTimeout %v", gqlExecTimeout, margin, HTTPWriteTimeout)
	}
}

// A resolver whose upstream call hit the execution deadline (here as the
// *url.Error net/http returns) reads as a named, retry-free QUERY_TIMEOUT, not
// a redacted "internal error" (w4/m140).
func TestSanitizeGraphQLErrorsNamesAQueryTimeout(t *testing.T) {
	upstream := &url.Error{Op: "Get", URL: "http://loki/loki/api/v1/query_range", Err: context.DeadlineExceeded}
	out := sanitizeGraphQLErrors([]gqlerrors.FormattedError{
		gqlerrors.FormatError(gqlerrors.NewError(upstream.Error(), nil, "", nil, nil, fmt.Errorf("query logs: %w", upstream))),
	})
	if out[0].Message != queryTimeoutMessage {
		t.Fatalf("message = %q, want the timeout guidance", out[0].Message)
	}
	if out[0].Extensions["code"] != core.CodeQueryTimeout {
		t.Fatalf("extensions = %v, want code %s", out[0].Extensions, core.CodeQueryTimeout)
	}
}
