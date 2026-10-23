package main

import (
	"context"
	"flag"

	"github.com/CodeTelemetry/terraform-provider-codetelemetry/terraform-provider-codetelemetry/internal/provider"
	"github.com/hashicorp/terraform-plugin-framework/providerserver"
)

var version = "dev"

func main() {
	debug := flag.Bool("debug", false, "Enable debugger support")
	flag.Parse()
	if err := providerserver.Serve(context.Background(), provider.New(version), providerserver.ServeOpts{
		Address: "registry.terraform.io/CodeTelemetry/codetelemetry", Debug: *debug,
	}); err != nil {
		panic(err)
	}
}
