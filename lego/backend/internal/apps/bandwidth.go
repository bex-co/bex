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
	"context"
	"errors"

	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// BandwidthErrorCode is the refusal for resuming a service suspended because
// its workspace used its included outbound bandwidth without a payment method
// (w6/m319).
const BandwidthErrorCode = "OUTBOUND_BANDWIDTH_EXHAUSTED"

// requireBandwidthAvailable refuses a manual resume of a service the spent
// included bandwidth suspended, while it is still that month and the workspace
// still has no payment method. With one bound, the resume goes ahead: the
// bandwidth beyond the allowance is billed from then on.
func (s *Service) requireBandwidthAvailable(ctx context.Context, a *appv1alpha1.App) error {
	month := a.Annotations[core.AnnotBandwidthSuspended]
	if month != FreeHoursMonth(s.Now()) {
		return nil
	}
	err := s.RequirePaymentMethod(ctx, a.Labels[core.LabelTenant])
	if !errors.Is(err, core.ErrPaymentRequired) {
		return err
	}
	return core.NewBadRequestError(BandwidthErrorCode,
		"this workspace has used the outbound bandwidth its plan includes this month; its services resume at the start of next month, or add a payment method to run them now",
		map[string]any{"month": month})
}
