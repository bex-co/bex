package main

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/render-oss/cli/pkg/client"
)

// TestSecretFileUpsertDecodesBex201 pins the consumer side of bex-api's
// service secret-file upsert: the pinned client fills its typed result only on
// the declared 201, and an empty file's content:"" must survive the decode
// (lego/backend/internal/secrets TestREST_EmptySecretFileKeepsContent; w8/073,
// w8/074). The 200 row is the old bex status, which decoded to no result.
func TestSecretFileUpsertDecodesBex201(t *testing.T) {
	for _, tc := range []struct {
		status  int
		body    string
		typed   bool
		content string
	}{
		{http.StatusCreated, `{"name":"empty.txt","content":""}`, true, ""},
		{http.StatusCreated, `{"name":"text.txt","content":"hi\n"}`, true, "hi\n"},
		{http.StatusOK, `{"name":"text.txt","content":"hi\n"}`, false, ""},
	} {
		resp, err := client.ParseAddOrUpdateSecretFileResponse(&http.Response{
			StatusCode: tc.status,
			Header:     http.Header{"Content-Type": {"application/json"}},
			Body:       io.NopCloser(strings.NewReader(tc.body)),
		})
		if err != nil {
			t.Fatal(err)
		}
		if (resp.JSON201 != nil) != tc.typed {
			t.Fatalf("status %d: typed result present = %v, want %v", tc.status, resp.JSON201 != nil, tc.typed)
		}
		if tc.typed && resp.JSON201.Content != tc.content {
			t.Fatalf("status %d: content %q not decoded from %s", tc.status, resp.JSON201.Content, tc.body)
		}
	}
}
