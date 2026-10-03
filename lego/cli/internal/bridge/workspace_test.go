package bridge

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/render-oss/cli/pkg/client"
)

func TestResolveWorkspaceName(t *testing.T) {
	owners := func(pairs ...string) []*client.Owner {
		var out []*client.Owner
		for i := 0; i < len(pairs); i += 2 {
			out = append(out, &client.Owner{Name: pairs[i], Id: pairs[i+1]})
		}
		return out
	}
	for _, tc := range []struct {
		name      string
		value     string
		listed    []*client.Owner
		listErr   error
		wantValue string
		wantErr   string
		wantCalls int
	}{
		{name: "unique name maps to id", value: "bex-canary",
			listed:    owners("bex-canary", "tea-daif693dqjvc73e7as3g", "bex-canary-2", "tea-other"),
			wantValue: "tea-daif693dqjvc73e7as3g", wantCalls: 1},
		{name: "id makes no request", value: "tea-daif693dqjvc73e7as3g",
			wantValue: "tea-daif693dqjvc73e7as3g"},
		{name: "unknown name names the workspace", value: "no-such-ws",
			listed: owners("bex-canary", "tea-a"), wantValue: "no-such-ws",
			wantErr: `no workspace named "no-such-ws"`, wantCalls: 1},
		{name: "duplicate names ask for the id", value: "shared",
			listed: owners("shared", "tea-a", "shared", "tea-b"), wantValue: "shared",
			wantErr: `several workspaces are named "shared" (tea-a, tea-b); use the id`, wantCalls: 1},
		{name: "no login passes the name through", value: "bex-canary",
			listErr: errors.New("run `render login` to authenticate"), wantValue: "bex-canary", wantCalls: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env := map[string]string{renderWorkspace: tc.value}
			calls := 0
			err := resolveWorkspaceName(context.Background(),
				func(k string) (string, bool) { v, ok := env[k]; return v, ok },
				func(k, v string) error { env[k] = v; return nil },
				func(_ context.Context, name string) ([]*client.Owner, error) {
					calls++
					if name != tc.value {
						t.Fatalf("listed %q, want %q", name, tc.value)
					}
					return tc.listed, tc.listErr
				})
			if tc.wantErr == "" && err != nil || tc.wantErr != "" && (err == nil || !strings.Contains(err.Error(), tc.wantErr)) {
				t.Fatalf("err = %v, want %q", err, tc.wantErr)
			}
			if env[renderWorkspace] != tc.wantValue || calls != tc.wantCalls {
				t.Fatalf("RENDER_WORKSPACE = %q (want %q), calls = %d (want %d)", env[renderWorkspace], tc.wantValue, calls, tc.wantCalls)
			}
		})
	}
}
