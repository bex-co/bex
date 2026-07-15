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
	"fmt"

	"github.com/bex-co/bex/lego/backend/internal/core"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func normalizeNotificationsToSend(value string) (string, error) {
	switch value {
	case "", appv1alpha1.NotificationsToSendDefault:
		return appv1alpha1.NotificationsToSendDefault, nil
	case appv1alpha1.NotificationsToSendNone,
		appv1alpha1.NotificationsToSendFailure,
		appv1alpha1.NotificationsToSendAll:
		return value, nil
	default:
		return "", fmt.Errorf("%w: notificationsToSend must be one of default|none|failure|all", core.ErrBadRequest)
	}
}
