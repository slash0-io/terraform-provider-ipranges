// terraform-provider-ipranges: Terraform data sources for third-party service
// IP ranges, backed by a versioned public feed of vendor-published ranges.
package main

import (
	"context"
	"flag"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"

	"github.com/slash0-io/terraform-provider-ipranges/internal/provider"
)

// version is set by goreleaser at build time.
var version = "dev"

func main() {
	var debug bool
	flag.BoolVar(&debug, "debug", false, "run with debugger support")
	flag.Parse()

	err := providerserver.Serve(context.Background(), provider.New(version), providerserver.ServeOpts{
		Address: "registry.terraform.io/slash0-io/ipranges",
		Debug:   debug,
	})
	if err != nil {
		log.Fatal(err)
	}
}
