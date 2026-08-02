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

package main

import (
	"context"
	"fmt"
	"os"

	"github.com/bex-co/bex/lego/backend/internal/agentcredential"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: git-credential-bex get|store|erase")
		os.Exit(2)
	}
	err := agentcredential.Run(context.Background(), os.Args[1], os.Stdin, os.Stdout, agentcredential.Config{
		GatewayURL: os.Getenv("BEX_AGENT_CREDENTIAL_URL"),
		Namespace:  os.Getenv("BEX_SANDBOX_NAMESPACE"),
		SessionID:  os.Getenv("BEX_AGENT_SESSION_ID"),
		Repository: os.Getenv("BEX_AGENT_REPOSITORY"),
		Branch:     os.Getenv("BEX_AGENT_BRANCH"),
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "git-credential-bex: %v\n", err)
		os.Exit(1)
	}
}
