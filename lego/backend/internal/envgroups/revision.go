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

package envgroups

import (
	"errors"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

const envGroupRevisionPrefix = "egr1_"

var errInvalidEnvGroupRevision = errors.New("invalid environment group revision")

func encodeEnvGroupRevision(version uint64) string {
	return core.EncodeOpaqueRevision(envGroupRevisionPrefix, version)
}

func decodeEnvGroupRevision(token string) (uint64, error) {
	version, ok := core.DecodeOpaqueRevision(envGroupRevisionPrefix, token)
	if !ok {
		return 0, errInvalidEnvGroupRevision
	}
	return version, nil
}
