package main

import (
	"github.com/hashicorp/terraform-plugin-sdk/v2/plugin"
	"github.com/rubrikinc/rubrik-provider-for-terraform/rubrikcdm"
)

func main() {
	plugin.Serve(&plugin.ServeOpts{
		ProviderFunc: rubrikcdm.Provider,
	})
}
