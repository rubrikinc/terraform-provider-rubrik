package main

import (
	"terraform-provider-rubrik-cdm/rubrikcdm"

	"github.com/hashicorp/terraform/plugin"
)

func main() {
	plugin.Serve(&plugin.ServeOpts{
		ProviderFunc: rubrikcdm.Provider,
	})
}

// func main() {
// 	plugin.Serve(&plugin.ServeOpts{
// 		ProviderFunc: func() terraform.ResourceProvider {
// 			return Provider()
// 		},
// 	})
// }
