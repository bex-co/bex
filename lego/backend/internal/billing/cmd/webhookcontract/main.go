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

// Command webhookcontract prints the Stripe webhook contract bex's receiver
// implements, so scripts/stripe-webhook-drift.sh can compare the live endpoint
// against the code instead of against a duplicated literal that would quietly
// rot. Read-only and dependency-free by design.
package main

import (
	"fmt"
	"os"

	stripe "github.com/stripe/stripe-go/v86"

	"github.com/bex-co/bex/lego/backend/internal/billing"
)

func main() {
	if len(os.Args) != 2 {
		fmt.Fprintln(os.Stderr, "usage: webhookcontract events|version")
		os.Exit(2)
	}
	switch os.Args[1] {
	case "events":
		for _, t := range billing.HandledStripeEventTypes() {
			fmt.Println(t)
		}
	case "version":
		fmt.Println(stripe.APIVersion)
	default:
		fmt.Fprintf(os.Stderr, "unknown subcommand %q\n", os.Args[1])
		os.Exit(2)
	}
}
