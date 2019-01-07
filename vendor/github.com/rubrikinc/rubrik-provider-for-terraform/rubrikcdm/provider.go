package rubrikcdm

import (
	"github.com/hashicorp/terraform/helper/schema"
	"github.com/hashicorp/terraform/helper/validation"
	"github.com/hashicorp/terraform/terraform"
)

// Provider returns a terraform.ResourceProvider.
func Provider() terraform.ResourceProvider {

	// The actual provider
	return &schema.Provider{
		Schema: map[string]*schema.Schema{
			"node_ip": {
				Type:         schema.TypeString,
				Required:     true,
				DefaultFunc:  schema.EnvDefaultFunc("rubrik_cdm_node_ip", nil),
				ValidateFunc: validation.SingleIP(),
				Description:  "The IP Address of a Node in the Rubrik cluster.",
			},
			"username": {
				Type:        schema.TypeString,
				Required:    true,
				DefaultFunc: schema.EnvDefaultFunc("rubrik_cdm_username", nil),
				Description: "The username used to authenticate against the Rubrik cluster.",
			},
			"password": {
				Type:        schema.TypeString,
				Required:    true,
				DefaultFunc: schema.EnvDefaultFunc("rubrik_cdm_password", nil),
				Description: "The password used to authenticate against the Rubrik cluster.",
			},
		},

		ResourcesMap: map[string]*schema.Resource{
			"rubrik_bootstrap":          resourceRubrikBootstrap(),
			"rubrik_cluster_version":    resourceRubrikClusterVersion(),
			"rubrik_configure_timezone": resourceRubrikConfigureTimezone(),
			"rubrik_aws_native_account": resourceRubrikAWSNativeAccount(),
			"rubrik_aws_s3_cloudout":    resourceRubrikAWSS3CloudOut(),
			"rubrik_aws_s3_cloudon":     resourceRubrikAWSS3CloudOn(),
			"rubrik_azure_cloudout":     resourceRubrikAzureCloudOut(),
			"rubrik_azure_cloudon":      resourceRubrikAzureCloudOn(),
		},

		ConfigureFunc: providerConfigure,
	}
}

func providerConfigure(d *schema.ResourceData) (interface{}, error) {
	config := Config{
		NodeIP:   d.Get("node_ip").(string),
		Username: d.Get("username").(string),
		Password: d.Get("password").(string),
	}

	return config.Client()
}
