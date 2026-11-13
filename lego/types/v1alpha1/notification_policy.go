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

package v1alpha1

const (
	NotificationsToSendDefault = "default"
	NotificationsToSendNone    = "none"
	NotificationsToSendFailure = "failure"
	NotificationsToSendAll     = "all"
)

// EffectiveNotificationsToSend resolves legacy Apps that predate the richer
// policy. notifyOnFail=notify meant "force failures", while ignore meant
// "suppress failures". Mapping those to failure/none preserves that intent.
func (s AppSpec) EffectiveNotificationsToSend() string {
	return EffectiveNotificationsToSendFor(s.NotificationsToSend, s.NotifyOnFail)
}

// EffectiveNotificationsToSendFor resolves an explicit service policy from the
// pair of fields, for callers that hold the strings rather than an AppSpec
// (bex-api's alert worker reads them off a selected App). One mapping serves
// the CR contract and notification delivery, so the two cannot drift.
//
// The legacy field only ever spoke about failures, so "notify" maps to
// "failure", not "all": both send the same failure mail, but only "all" opts
// into non-failure mail such as a disk back below its threshold.
func EffectiveNotificationsToSendFor(notificationsToSend, notifyOnFail string) string {
	if notificationsToSend != "" {
		return notificationsToSend
	}
	switch notifyOnFail {
	case "notify":
		return NotificationsToSendFailure
	case "ignore":
		return NotificationsToSendNone
	default:
		return NotificationsToSendDefault
	}
}

// NotifyOnFailForNotificationsToSend keeps Render's read-side convenience
// field compatible while notificationsToSend remains authoritative.
func NotifyOnFailForNotificationsToSend(policy string) string {
	switch policy {
	case NotificationsToSendNone:
		return "ignore"
	case NotificationsToSendFailure, NotificationsToSendAll:
		return "notify"
	default:
		return "default"
	}
}
