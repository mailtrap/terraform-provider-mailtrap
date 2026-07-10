package main

import (
	"context"
	"flag"
	"log"

	"github.com/hashicorp/terraform-plugin-framework/providerserver"

	"github.com/mailtrap/terraform-provider-mailtrap/internal/provider"
)

// version is overridden at release time via -ldflags; "dev" for local builds.
var version = "dev"

func main() {
	var debug bool
	flag.BoolVar(&debug, "debug", false, "run the provider with support for debuggers like delve")
	flag.Parse()

	err := providerserver.Serve(context.Background(), provider.New(version), providerserver.ServeOpts{
		Address: "registry.terraform.io/mailtrap/mailtrap",
		Debug:   debug,
	})
	if err != nil {
		log.Fatal(err.Error())
	}
}
