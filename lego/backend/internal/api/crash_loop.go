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
	"github.com/bex-co/bex/lego/backend/internal/crashloop"
	"github.com/bex-co/bex/lego/backend/internal/notifications"
)

// newCrashLoop wires the crash-loop suspension sweeper. The suspension runs
// whenever Apps are reachable; the email needs the control-plane store.
func newCrashLoop(appsSvc *apps.Service, d Deps) *crashloop.Sweeper {
	window := d.CrashLoopSuspendAfter
	if window <= 0 {
		window = crashloop.DefaultWindow
	}
	w := &crashloop.Sweeper{Apps: apps.CrashLoopWorker{Svc: appsSvc}, Window: window}
	if alerts, ok := d.Store.(notifications.CrashLoopAlertStore); ok && appsSvc != nil && appsSvc.Base != nil {
		w.Notifier = notifications.CrashLoopNotifier{
			Store: alerts, Policy: notifications.AppNotificationPolicy(appsSvc.Client), Window: window,
		}
	}
	return w
}
