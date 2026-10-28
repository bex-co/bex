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

package execution

import (
	"testing"

	"github.com/bex-co/bex/lego/operator/internal/execution/executiontest"
)

func TestKubernetesLiteralSurvivesOneKubeletExpansion(t *testing.T) {
	known := map[string]string{"HOME": "/builder/home", "PORT": "10000", "EMPTY": "", "NESTED": "$(PORT)"}
	for _, literal := range []string{
		"", "plain", "$", "$λ", "$(变量)", "$$", "a$$b", "$(PORT)", "$$(PORT)", "$$$(PORT)", "$$$$$(PORT)",
		"$(UNKNOWN)", "$(EMPTY)", "$(NESTED)", "$(unfinished", "$PORT ${PORT} \\$PORT", "first\n$(HOME)\n$$\n",
	} {
		if got := executiontest.Expand(KubernetesLiteral(literal), known); got != literal {
			t.Fatalf("literal=%q after kubelet=%q", literal, got)
		}
	}
}
