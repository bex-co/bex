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
		Apps:        apps.FreeHoursWorker{Svc: appsSvc},
		Store:       accounting,
		Notifier:    notifications.FreeHoursNotifier{Service: notificationsSvc},
		BudgetHours: d.FreeInstanceHours,
	}
}
