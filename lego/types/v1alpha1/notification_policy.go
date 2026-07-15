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
	if s.NotificationsToSend != "" {
		return s.NotificationsToSend
	}
	switch s.NotifyOnFail {
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
