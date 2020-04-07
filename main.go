package main

import (
	"github.com/hashicorp/terraform-plugin-sdk/plugin"
	"github.com/rubrikinc/terraform-provider-rubrik/rubrik"
)

func main() {
	plugin.Serve(&plugin.ServeOpts{
		ProviderFunc: rubrikcdm.Provider,
	})
}
