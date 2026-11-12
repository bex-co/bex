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

import (
	"crypto/sha256"
	"fmt"
)

// The suffixes of the Ingresses an App owns beside its serving Ingress (named
// after the App): the auto-paired www/apex redirects (w6/m23) and its wildcard
// custom domains' low-priority route (w4/m326).
const (
	HostRedirectsIngressSuffix = "-host-redirects"
	WildcardIngressSuffix      = ".wildcard"
)

// ChildIngressName is appName+suffix, shortened with a hash of appName when
// that exceeds a DNS label: the operator writes the Ingress under it, and
// bex-api finds it there to meter and chart the App's traffic.
func ChildIngressName(appName, suffix string) string {
	if len(appName)+len(suffix) <= 63 {
		return appName + suffix
	}
	sum := fmt.Sprintf("%x", sha256.Sum256([]byte(appName)))[:8]
	return appName[:63-len(suffix)-len(sum)-1] + "-" + sum + suffix
}

// ChildIngressNames are the names of every Ingress an App may own beside its
// serving one.
func ChildIngressNames(appName string) []string {
	return []string{
		ChildIngressName(appName, HostRedirectsIngressSuffix),
		ChildIngressName(appName, WildcardIngressSuffix),
	}
}
