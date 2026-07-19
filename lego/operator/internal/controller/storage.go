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

package controller

import "k8s.io/apimachinery/pkg/api/resource"

const bytesPerGi = int64(1024 * 1024 * 1024)

func quantityGiCeil(q resource.Quantity) int32 {
	bytes := q.Value()
	if bytes <= 0 {
		return 0
	}
	return int32((bytes + bytesPerGi - 1) / bytesPerGi)
}

func storageQuantityGB(value string) int32 {
	q, err := resource.ParseQuantity(value)
	if err != nil {
		return 0
	}
	return quantityGiCeil(q)
}
