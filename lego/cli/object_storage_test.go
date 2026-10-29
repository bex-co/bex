package main_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
)

// w6/097: bex has no Render early-access object storage (ADR018). Every cloud
// `ea objects` call says so without sending a request; --local keeps reaching
// upstream's own handling.
func TestEaObjectsSaysObjectStorageIsNotOffered(t *testing.T) {
	var requests atomic.Int32
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/v1/objects") {
			requests.Add(1)
		}
		http.NotFound(w, r)
	}))
	defer api.Close()
	for _, args := range [][]string{
		{"ea", "objects", "list", "--region", "oregon"},
		{"ea", "objects", "get", "dumps/a", "--region", "frankfurt", "--file", "out"},
		{"ea", "objects", "delete", "dumps/a", "--region", "fsn1", "--yes"},
	} {
		command := bexAgainst(t, api.URL, args...)
		var stderr bytes.Buffer
		command.Stderr = &stderr
		if err := command.Run(); err == nil || !strings.Contains(stderr.String(), "bex does not offer Render's early-access object storage") {
			t.Errorf("%v: err=%v stderr=%q, want a refusal naming that object storage is not offered", args, err, stderr.String())
		}
	}
	if n := requests.Load(); n != 0 {
		t.Errorf("sent %d object storage requests, want none", n)
	}
	for _, local := range []*exec.Cmd{
		bexAgainst(t, api.URL, "ea", "objects", "list", "--region", "oregon", "--local"),
		bexAgainst(t, api.URL, "ea", "objects", "list", "--region", "oregon"),
	} {
		if !slices.Contains(local.Args, "--local") {
			local.Env = append(local.Env, "RENDER_USE_LOCAL_DEV=true")
		}
		local.Dir = t.TempDir()
		if out, _ := local.CombinedOutput(); strings.Contains(string(out), "does not offer") {
			t.Fatalf("%v (local mode) was refused: %s", local.Args, out)
		}
	}
}
