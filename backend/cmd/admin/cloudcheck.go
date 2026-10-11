package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/quixgit/greenops-reliabilix/backend/internal/cloudaccounts/domain"
	"github.com/quixgit/greenops-reliabilix/backend/internal/cloudaccounts/providers/aws"
	"github.com/quixgit/greenops-reliabilix/backend/internal/cloudaccounts/providers/gcp"
	"github.com/quixgit/greenops-reliabilix/backend/internal/platform/focus"
)

// runCloudCheck: `admin aws-check <account id> <role arn> <external id>` and
// `admin gcp-check <project id> <bq://project/dataset/table> <external id>`. It runs what Verify and a sync do against
// one connection with the platform identity of this machine (the default AWS / Google credential chain), and prints
// the step, what came back and, on failure, the exact text the customer would see on the connection.
//
// A connection made with the customer's own credential: pass `awskey://<access key id>` (AWS) or a normal bq:// table
// (GCP) and put the secret access key, or the service account JSON, in CLOUD_CHECK_SECRET (or its file in
// CLOUD_CHECK_KEY_FILE).
func runCloudCheck(kind string, args []string) int {
	if len(args) != 3 {
		fmt.Fprintf(os.Stderr, "usage: admin %s-check <account or project id> <role arn | bq://project/dataset/table> <external id>\n", kind)
		return 2
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	// A customer's own credential (AWS secret access key, Google service account key file) comes from the environment,
	// never from an argument, so it does not land in a shell history or a process list.
	own := os.Getenv("CLOUD_CHECK_SECRET")
	if f := os.Getenv("CLOUD_CHECK_KEY_FILE"); f != "" && own == "" {
		b, err := os.ReadFile(f)
		if err != nil {
			fmt.Println("key file:", err)
			return 2
		}
		own = string(b)
	}
	secret := func(domain.Connection) (string, error) { return own, nil }
	keyMode := own != ""
	c := domain.Connection{ProjectID: "check", AccountRef: args[0], CredentialRef: args[1], ExternalID: args[2]}
	var p interface {
		domain.CloudProvider
		domain.ControlProvider
	}
	switch kind {
	case "aws":
		c.Provider = domain.AWS
		ap, err := aws.New(ctx, secret)
		if err != nil {
			fmt.Println("platform identity: FAILED:", err)
			return 1
		}
		p = ap
	case "gcp":
		c.Provider = domain.GCP
		if keyMode {
			c.SealedSecret = "from-environment" // marks the key mode; the provider asks secret() for the key
			p = gcp.NewKeyOnly(secret)
			break
		}
		gp, err := gcp.New(ctx, os.Getenv("PLATFORM_GCP_PROJECT"))
		if err != nil {
			fmt.Println("platform identity: FAILED:", err)
			return 1
		}
		p = gp.WithKeys(secret)
	}
	if err := c.Validate(); err != nil {
		fmt.Println("1. the connection as entered: FAILED:", err)
		return 1
	}
	ok := true
	step := func(name string, err error) bool {
		if err != nil {
			fmt.Printf("%s FAILED: %v\n", name, err)
			if domain.NeedsCustomerAction(err) {
				fmt.Println("   (the customer must act; retrying cannot help)")
			} else {
				fmt.Println("   (temporary: the sync would be retried)")
			}
			ok = false
			return false
		}
		fmt.Printf("%s ok\n", name)
		return true
	}
	step("1. verify (access, ownership, one-day query)", p.Validate(ctx, c))
	day := time.Now().UTC().Truncate(24 * time.Hour)
	req := domain.UsageRequest{Connection: c, From: day.AddDate(0, 0, -7), To: day}
	batches, err := p.GetUsage(ctx, req)
	if step("2. read the last 7 days", err) {
		for _, b := range batches {
			fmt.Printf("   batch source=%s, %d bytes\n", b.Source, len(b.Payload))
		}
	}
	totals, err := p.GetControlTotals(ctx, req)
	if step("3. control totals (the figures the stored data is reconciled with)", err) {
		printTotals(totals)
	}
	if ok {
		fmt.Println("\nAll steps worked: the platform can read this account's costs.")
		return 0
	}
	fmt.Println("\nAt least one step failed: send the lines above (they contain no secret).")
	return 1
}

func printTotals(t []focus.ControlTotal) {
	for i, x := range t {
		if i == 7 {
			break
		}
		fmt.Printf("   %s %s %.2f\n", x.Day, x.Currency, x.Billed)
	}
}
