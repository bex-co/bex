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

package signins

import (
	"fmt"
	"log"

	"github.com/graphql-go/graphql"

	"github.com/bex-co/bex/lego/backend/internal/core"
)

// errSignOutUnavailable answers a sign-out report whose rows did not land.
var errSignOutUnavailable = core.Unavailable("sign-out audit unavailable")

// GraphQLMutation contributes recordSignOut, the dashboard's report that the
// member is signing out, sent just before it ends the Kratos session. Only a
// member's own browser session may report it, and it records Render's
// LogoutEvent in each of their workspaces. The dashboard does not wait for it
// past a short timeout, so a failure never keeps anyone signed in.
func (r *Recorder) GraphQLMutation() graphql.Fields {
	return graphql.Fields{
		"recordSignOut": &graphql.Field{
			Type:        graphql.NewNonNull(graphql.Boolean),
			Description: "Records that the signed-in member is signing out of the dashboard: Render's LogoutEvent in each of their workspaces' audit logs. Call it before ending the session.",
			Resolve: func(p graphql.ResolveParams) (any, error) {
				caller, ok := core.IdentityFrom(p.Context)
				if !ok || !caller.Human || caller.Method != core.MethodSession {
					return nil, fmt.Errorf("%w: session authentication required", core.ErrForbidden)
				}
				if err := r.RecordSignOut(p.Context, caller.Subject, caller.Method); err != nil {
					log.Printf("signins: record the sign-out of %s: %v", caller.Subject, err)
					return nil, errSignOutUnavailable
				}
				return true, nil
			},
		},
	}
}
