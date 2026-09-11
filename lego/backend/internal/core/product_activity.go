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

package core

import (
	"context"
	"log"
	"time"
)

// ProductActivity is a successful resource effect, not an authorization check.
// There is deliberately no free-form metadata, name, URL or error message.
type ProductActivity struct {
	WorkspaceID  string
	ResourceID   string
	ParentID     string
	ResourceType string
	EventType    string
	At           time.Time
	ActorID      string
	ActorType    string
	// Surface is set by ObserveProductActivity from the request context;
	// callers must not populate it, as it is overwritten.
	Surface string
}

// ObserveProductActivity is best effort and bounded; analytics must never turn
// a completed resource creation into an API error or replay the mutation.
func (b *Base) ObserveProductActivity(ctx context.Context, event ProductActivity) {
	if b.ProductActivity == nil || event.WorkspaceID == "" || event.ResourceID == "" {
		return
	}
	if event.At.IsZero() {
		event.At = b.Now()
	}
	// Resolved here rather than at the call sites: every create tail already
	// carries the request context, so none of them need to know the rules.
	event.Surface = validSurface(ProductSurface(ctx))
	event.ActorType = "unknown"
	if identity, ok := IdentityFrom(ctx); ok && identity.Subject != "" {
		event.ActorID = identity.Subject
		switch {
		case identity.Method == "session" || identity.Human:
			event.ActorType = "human"
		case identity.Method == "oauth2":
			event.ActorType = "machine"
		}
	}
	bounded, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Second)
	defer cancel()
	if err := b.ProductActivity(bounded, event); err != nil {
		log.Printf("product analytics: record %s: %v", event.EventType, err)
	}
}
