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
	"github.com/bex-co/bex/lego/backend/internal/apps"
	"github.com/bex-co/bex/lego/backend/internal/bandwidth"
	"github.com/bex-co/bex/lego/backend/internal/buildminutes"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/freehours"
	"github.com/bex-co/bex/lego/backend/internal/notifications"
)

// newFreeHours wires the monthly Free instance-hours sweeper. It does nothing
// unless the intent store can account the meter (the production PGStore).
func newFreeHours(appsSvc *apps.Service, d Deps, notificationsSvc *notifications.Service) *freehours.Sweeper {
	accounting, ok := d.Store.(freehours.Store)
	if !ok {
		return nil
	}
	return &freehours.Sweeper{
		Apps:        apps.BillingSuspensionWorker{Svc: appsSvc, Annotation: core.AnnotFreeHoursSuspended},
		Store:       accounting,
		Notifier:    notifications.FreeHoursNotifier{Service: notificationsSvc},
		BudgetHours: d.FreeInstanceHours,
	}
}

// newBuildMinutes wires the monthly build-minutes sweeper (w6/m308): with Stripe
// billing on, a workspace that has used its plan's included pipeline minutes
// and cannot pay for more stops building until the month turns. Without
// billing (self-hosted) nothing is metered for a price, so nothing stops.
func newBuildMinutes(appsSvc *apps.Service, d Deps, notificationsSvc *notifications.Service) *buildminutes.Sweeper {
	accounting, ok := d.Store.(buildminutes.Store)
	if !ok || d.Billing == nil {
		return nil
	}
	return &buildminutes.Sweeper{
		Apps:     apps.BuildMinutesWorker{Svc: appsSvc},
		Store:    accounting,
		Notifier: notifications.BuildMinutesNotifier{Service: notificationsSvc},
		Plans:    appsSvc.Base,
	}
}

// newBandwidth wires the monthly included-bandwidth sweeper (w6/m319): with
// Stripe billing on, a workspace that has used its plan's outbound bandwidth
// and has no payment method has its services spun down until the month turns.
// It also needs the payment-method gate (BEX_REQUIRE_PAYMENT_METHOD): that
// gate refuses the manual resume, and without it a resume and the next sweep
// would undo each other.
func newBandwidth(appsSvc *apps.Service, d Deps, notificationsSvc *notifications.Service) *bandwidth.Sweeper {
	accounting, ok := d.Store.(bandwidth.Store)
	if !ok || d.Billing == nil || appsSvc.Payment == nil {
		return nil
	}
	return &bandwidth.Sweeper{
		Apps:     apps.BillingSuspensionWorker{Svc: appsSvc, Annotation: core.AnnotBandwidthSuspended},
		Store:    accounting,
		Notifier: notifications.BandwidthNotifier{Service: notificationsSvc},
		Plans:    appsSvc.Base,
	}
}
