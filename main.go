package main

import (
	"github.com/46bit/terraform-provider-parameterstore/aws"
	"github.com/hashicorp/terraform/plugin"
)

func main() {
	plugin.Serve(&plugin.ServeOpts{
		ProviderFunc: aws.Provider})
}
