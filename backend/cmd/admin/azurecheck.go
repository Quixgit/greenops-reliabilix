package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/cloudaccounts/providers/azure"
)

// runAzureCheck: `AZURE_CLIENT_SECRET=... admin azure-check <subscription id> <tenant id> <application id>`.
// It signs in as the service principal and runs the queries of a sync, printing what Azure answers. The secret is read
// from the environment so it never lands in a shell history or a process list.
func runAzureCheck(args []string) int {
	secret := os.Getenv("AZURE_CLIENT_SECRET")
	if len(args) != 3 || secret == "" {
		fmt.Fprintln(os.Stderr, "usage: AZURE_CLIENT_SECRET='<secret value>' admin azure-check <subscription id> <tenant id> <application (client) id>")
		return 2
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	if azure.Diagnose(ctx, os.Stdout, args[1], args[2], secret, args[0]) {
		fmt.Println("\nAll steps worked: the platform can read this subscription's costs.")
		return 0
	}
	fmt.Println("\nAt least one step failed: send the lines above (they contain no secret).")
	return 1
}
