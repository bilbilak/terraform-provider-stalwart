package main

import (
	"context"
	"flag"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"
	"github.com/bilbilak/terraform-provider-stalwart/internal/provider"
)

// version is set to "1.2.0" by default and overridden by GoReleaser via ldflags
// during release builds: -X main.version={{.Version}}
var version = "1.2.0"

func main() {
	var debug bool
	flag.BoolVar(&debug, "debug", false, "run provider with debugger support")
	flag.Parse()

	err := providerserver.Serve(context.Background(), provider.New(version), providerserver.ServeOpts{
		Address: "registry.terraform.io/bilbilak/stalwart",
		Debug:   debug,
	})
	if err != nil {
		log.Fatal(err)
	}
}
