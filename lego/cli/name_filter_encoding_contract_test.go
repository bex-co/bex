package main

import (
	"testing"

	"github.com/render-oss/cli/pkg/client"
)

// TestServiceNameFilterRawEncoding pins the bytes the pinned client's generated
// NewListServicesRequest puts on the wire for `services update|create --from
// <name>`: raw commas separate array elements, while a comma inside a name is
// percent-encoded. bex-api's core.QueryNameList splits on exactly this
// distinction, and lego/backend/internal/apps/name_filter_comma_test.go replays
// these raw queries against the composed list endpoint (w8/m55). A pin bump
// that changes this encoding fails here first.
func TestServiceNameFilterRawEncoding(t *testing.T) {
	for _, tc := range []struct {
		names client.NameParam
		raw   string
	}{
		{client.NameParam{"qa-literal,雪%+&="}, "name=qa-literal%2C%E9%9B%AA%25%2B%26%3D"},
		{client.NameParam{"qa-first", "qa-second"}, "name=qa-first,qa-second"},
		{client.NameParam{"qa-first", "qa-literal,雪%+&="}, "name=qa-first,qa-literal%2C%E9%9B%AA%25%2B%26%3D"},
		{client.NameParam{"qa-literal%2C"}, "name=qa-literal%252C"},
	} {
		req, err := client.NewListServicesRequest("https://api.bex.co/v1/", &client.ListServicesParams{Name: &tc.names})
		if err != nil {
			t.Fatal(err)
		}
		if req.URL.RawQuery != tc.raw {
			t.Errorf("names %q encode as %q, want %q", tc.names, req.URL.RawQuery, tc.raw)
		}
	}
}
