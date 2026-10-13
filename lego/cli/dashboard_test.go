package main

import (
	"testing"

	"github.com/render-oss/cli/cmd"
)

// w8/090: only the hosting commands with a Dashboard action seed the origin,
// resolved through cobra so the service alias counts.
func TestOpensDashboard(t *testing.T) {
	for _, tc := range []struct {
		args []string
		want bool
	}{
		{[]string{"services"}, true},
		{[]string{"service"}, true},
		{[]string{"services", "--confirm", "-o", "interactive"}, true},
		{[]string{"deploys", "list", "srv-x"}, true},
		{[]string{"deploys"}, false},
		{[]string{"whoami"}, false},
		{[]string{"logs"}, false},
		{[]string{"version"}, false},
		{nil, false},
	} {
		if got := opensDashboard(cmd.RootCmd, tc.args); got != tc.want {
			t.Errorf("opensDashboard(%q) = %v, want %v", tc.args, got, tc.want)
		}
	}
}
