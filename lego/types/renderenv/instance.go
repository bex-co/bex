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

package renderenv

import (
	"crypto/sha256"
	"encoding/base32"
)

// InstanceIDAnnotation is the pod annotation that carries the pod's public
// instance id. The operator's admission webhook stamps it at creation, and the
// container reads it as RENDER_INSTANCE_ID through the downward API (w4/386).
const InstanceIDAnnotation = "app.bex.co/instance-id"

// instanceEncoding is lowercase base32-hex, the alphabet of bex ids.
var instanceEncoding = base32.NewEncoding("0123456789abcdefghijklmnopqrstuv").WithPadding(base32.NoPadding)

// PublicInstanceID is the id bex reports for the pod named podName of service
// serviceID: serviceID, "-", then 20 base32-hex characters of sha256(podName).
// It is the id GET /v1/services/{id}/instances, logs and metrics carry
// (lego/backend/internal/id.ServiceInstanceID, w4/m87); a backend test pins the
// two derivations together. Only the pod name goes in, so it can be computed
// before the pod exists, at admission.
func PublicInstanceID(serviceID, podName string) string {
	sum := sha256.Sum256([]byte(podName))
	return serviceID + "-" + instanceEncoding.EncodeToString(sum[:13])[:20]
}
