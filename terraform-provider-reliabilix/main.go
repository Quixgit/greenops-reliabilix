// terraform-provider-reliabilix manages Reliabilix projects, their policies and budgets as code.
package main

import (
	"context"
	"flag"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"

	"github.com/quixgit/greenops-reliabilix/terraform-provider-reliabilix/internal/provider"
)

// version is set by the release build (-ldflags "-X main.version=...").
var version = "0.1.0-alpha.168"

func main() {
	var debug bool
	flag.BoolVar(&debug, "debug", false, "run the provider with debugger support")
	flag.Parse()
	err := providerserver.Serve(context.Background(), provider.New(version), providerserver.ServeOpts{Address: "registry.terraform.io/reliabilix/reliabilix", Debug: debug})
	if err != nil {
		log.Fatal(err)
	}
}
