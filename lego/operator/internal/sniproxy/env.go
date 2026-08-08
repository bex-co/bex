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

package sniproxy

import (
	"os"
	"strconv"
	"time"

	"github.com/go-logr/logr"
)

// EnvInt reads an integer env var, falling back to def when unset or malformed
// (a malformed value is logged, never fatal — these are tuning knobs).
func EnvInt(log logr.Logger, name string, def int) int {
	raw := os.Getenv(name)
	if raw == "" {
		return def
	}
	n, err := strconv.Atoi(raw)
	if err != nil {
		log.Error(err, "invalid integer env var; using default", "var", name, "default", def)
		return def
	}
	return n
}

// EnvDuration reads a Go duration env var, falling back to def when unset or
// malformed (a malformed value is logged, never fatal).
func EnvDuration(log logr.Logger, name string, def time.Duration) time.Duration {
	raw := os.Getenv(name)
	if raw == "" {
		return def
	}
	d, err := time.ParseDuration(raw)
	if err != nil {
		log.Error(err, "invalid duration env var; using default", "var", name, "default", def)
		return def
	}
	return d
}
