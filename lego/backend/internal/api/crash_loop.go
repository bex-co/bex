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
	"github.com/bex-co/bex/lego/backend/internal/diskusage"
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
	if alerts, ok := d.Store.(notifications.ServiceAlertStore); ok && appsSvc != nil && appsSvc.Base != nil {
		w.Notifier = notifications.CrashLoopNotifier{
			Store: alerts, Policy: notifications.AppNotificationPolicy(appsSvc.Client), Window: window,
		}
	}
	return w
}

// newDiskUsage wires the service-disk usage sweeper (w2/m255). It needs the
// control-plane store (disks, state, facts) and the kubelet volume-stats
// source; without either it does nothing.
func newDiskUsage(appsSvc *apps.Service, d Deps) *diskusage.Sweeper {
	st, ok := d.Store.(diskusage.Store)
	if !ok || d.DiskUsage == nil || appsSvc == nil || appsSvc.Base == nil {
		return nil
	}
	w := &diskusage.Sweeper{Apps: appsSvc.Client, Store: st, Usage: d.DiskUsage}
	if alerts, ok := d.Store.(notifications.ServiceAlertStore); ok {
		w.Notifier = notifications.DiskUsageNotifier{Store: alerts, Policy: notifications.AppNotificationPolicy(appsSvc.Client)}
	}
	return w
}
