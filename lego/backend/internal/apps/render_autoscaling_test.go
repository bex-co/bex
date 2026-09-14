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

package apps

import (
	"errors"
	"testing"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

func TestSetAutoscalingRequestFromRender(t *testing.T) {
	t.Run("maps Render criteria onto SetAutoscalingRequest", func(t *testing.T) {
		req, enable, err := setAutoscalingRequestFromRender(renderAutoscalingConfig{
			Enabled: true,
			Min:     1,
			Max:     3,
			Criteria: renderAutoscalingCriteria{
				CPU:    renderAutoscalingCriterion{Enabled: true, Percentage: 60},
				Memory: renderAutoscalingCriterion{Enabled: false, Percentage: 0},
			},
		})
		if err != nil || !enable {
			t.Fatalf("map: enable=%v err=%v", enable, err)
		}
		if req.MinInstances != 1 || req.MaxInstances != 3 {
			t.Fatalf("instances = %d/%d", req.MinInstances, req.MaxInstances)
		}
		if req.TargetCPUPercent == nil || *req.TargetCPUPercent != 60 {
			t.Fatalf("cpu = %v", req.TargetCPUPercent)
		}
		if req.TargetMemoryPercent != nil {
			t.Fatalf("memory should be unset, got %v", *req.TargetMemoryPercent)
		}
	})

	t.Run("enabled false means disable", func(t *testing.T) {
		_, enable, err := setAutoscalingRequestFromRender(renderAutoscalingConfig{Enabled: false})
		if err != nil || enable {
			t.Fatalf("enable=%v err=%v", enable, err)
		}
	})

	t.Run("no enabled criterion is a named bad request", func(t *testing.T) {
		_, enable, err := setAutoscalingRequestFromRender(renderAutoscalingConfig{
			Enabled: true, Min: 1, Max: 1,
			Criteria: renderAutoscalingCriteria{
				CPU:    renderAutoscalingCriterion{Enabled: false, Percentage: 60},
				Memory: renderAutoscalingCriterion{Enabled: false, Percentage: 40},
			},
		})
		if !enable || !errors.Is(err, core.ErrBadRequest) {
			t.Fatalf("enable=%v err=%v", enable, err)
		}
		if err == nil || err.Error() == "bad request" {
			t.Fatal("want named reason, not bare bad request")
		}
	})
}

func TestRenderAutoscalingConfigRoundTrip(t *testing.T) {
	cpu := int32(60)
	view := AutoscalingView{Enabled: true, MinInstances: 1, MaxInstances: 2, TargetCPUPercent: &cpu}
	wire := renderAutoscalingConfigFromView(view)
	if !wire.Enabled || wire.Min != 1 || wire.Max != 2 {
		t.Fatalf("wire = %+v", wire)
	}
	if !wire.Criteria.CPU.Enabled || wire.Criteria.CPU.Percentage != 60 {
		t.Fatalf("cpu = %+v", wire.Criteria.CPU)
	}
	if wire.Criteria.Memory.Enabled {
		t.Fatalf("memory should be disabled: %+v", wire.Criteria.Memory)
	}
	back, enable, err := setAutoscalingRequestFromRender(wire)
	if err != nil || !enable {
		t.Fatalf("back: %v %v", enable, err)
	}
	if back.MinInstances != 1 || back.MaxInstances != 2 || back.TargetCPUPercent == nil || *back.TargetCPUPercent != 60 {
		t.Fatalf("back = %+v", back)
	}
}
