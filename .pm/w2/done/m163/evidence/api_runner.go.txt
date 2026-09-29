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
// Temporary m163 live acceptance runner; never shipped.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	"github.com/bex-co/bex/lego/backend/internal/accounts"
	"github.com/bex-co/bex/lego/backend/internal/api"
	"github.com/bex-co/bex/lego/backend/internal/apikeys"
	"github.com/bex-co/bex/lego/backend/internal/authz"
	"github.com/bex-co/bex/lego/backend/internal/core"
	"github.com/bex-co/bex/lego/backend/internal/members"
	"github.com/bex-co/bex/lego/backend/internal/store"
	"github.com/bex-co/bex/lego/backend/internal/workspaces"
	appv1alpha1 "github.com/bex-co/bex/lego/types/v1alpha1"
)

func main() {
	ctx := context.Background()
	uri := os.Getenv("BEX_CP_DB_URI")
	if uri == "" || os.Getenv("BEX_OPENFGA_URL") == "" {
		log.Fatal("real Postgres and OpenFGA are required")
	}
	if err := store.Migrate(uri); err != nil {
		log.Fatal(err)
	}
	pool, err := pgxpool.New(ctx, uri)
	if err != nil {
		log.Fatal(err)
	}
	defer pool.Close()
	st := store.NewPGStore(pool)
	checker := authz.NewOpenFGAChecker(os.Getenv("BEX_OPENFGA_URL"), os.Getenv("BEX_OPENFGA_TOKEN"))
	roles := checker.(store.MembershipGranter)
	resolver := api.NewTenantService(st, roles)
	resolver.Audit = st
	resolver.RequireVerifiedInviteEmail = true
	scheme := runtime.NewScheme()
	if err := clientgoscheme.AddToScheme(scheme); err != nil {
		log.Fatal(err)
	}
	if err := appv1alpha1.AddToScheme(scheme); err != nil {
		log.Fatal(err)
	}
	base := &core.Base{Client: fake.NewClientBuilder().WithScheme(scheme).Build(), Namespace: "dev-2", Authz: checker, Workspace: resolver, Audit: st}
	cleaner := accounts.NewOryCleaner(os.Getenv("BEX_HYDRA_ADMIN_URL"), os.Getenv("BEX_KRATOS_ADMIN_URL"))
	srv := api.NewServer(base, api.Deps{
		Store: st, WorkspaceStore: st, MembersStore: st, AccountStore: st,
		WorkspaceCreationStore: st,
		APIKeys:                apikeys.NewHydraAPIKeys(os.Getenv("BEX_HYDRA_ADMIN_URL")),
		MembersGranter:         checker.(members.RoleGranter), MembersRevoker: checker.(members.RoleRevoker),
		WorkspaceGranter: checker.(workspaces.WorkspaceGranter), WorkspaceRevoker: checker.(workspaces.WorkspaceRevoker),
		KeyBinder: resolver, Onboard: resolver, OAuthRevocations: st,
		Identities:   workspaces.NewKratosIdentities(os.Getenv("BEX_KRATOS_ADMIN_URL")),
		AccountOAuth: cleaner, AccountKratos: cleaner,
	})
	srv.HydraAdminURL = os.Getenv("BEX_HYDRA_ADMIN_URL")
	srv.KratosURL = os.Getenv("BEX_KRATOS_URL")
	srv.OAuthIssuer = os.Getenv("BEX_OAUTH_ISSUER")
	handler, err := srv.Handler()
	if err != nil {
		log.Fatal(err)
	}
	go srv.Members.RunRoleReconciler(ctx)
	go srv.Accounts.Run(ctx)
	server := &http.Server{Addr: "127.0.0.1:54020", Handler: handler, ReadHeaderTimeout: 10 * time.Second}
	log.Print("m163 acceptance: real PostgreSQL/Hydra/Kratos/OpenFGA; fake Kubernetes; listening on 127.0.0.1:54020")
	log.Fatal(server.ListenAndServe())
}
