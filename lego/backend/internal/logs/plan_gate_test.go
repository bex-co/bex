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

package logs

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// w8/m138: HTTP request logs are a Pro-and-higher feature. On Hobby an explicit
// type=request is refused with the plan error; an all-types query keeps
// answering with the app lines.
func TestRequestLogsFollowThePlan(t *testing.T) {
	app := sampleApp("web")
	app.Labels = map[string]string{core.LabelTenant: "tea-a"}
	lines := map[string][]string{webInst: {"2026-07-05T00:00:01Z hello"}}

	for plan, refused := range map[string]bool{"hobby": true, "pro": false} {
		t.Run(plan, func(t *testing.T) {
			svc := newService(lines, app, podFor("web", webInst))
			svc.Plans = core.WorkspacePlanFunc(func(context.Context, string) (string, error) { return plan, nil })

			_, err := svc.QueryLogs(context.Background(), LogQuery{App: "web", Types: []string{LogTypeRequest}})
			if refused {
				if !errors.Is(err, core.ErrBadRequest) || !strings.Contains(err.Error(), "HTTP request logs require the Pro plan") {
					t.Fatalf("hobby: want the Pro-plan refusal, got %v", err)
				}
			} else if errors.Is(err, core.ErrBadRequest) {
				t.Fatalf("pro: request logs must not be plan-refused: %v", err)
			}

			entries, err := svc.QueryLogs(context.Background(), LogQuery{App: "web"})
			if err != nil || len(entries) == 0 {
				t.Fatalf("%s: app lines must keep answering: %v (%d lines)", plan, err, len(entries))
			}
		})
	}
}
