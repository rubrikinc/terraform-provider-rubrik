package main

import (
	"github.com/hashicorp/terraform/helper/schema"
	"github.com/hashicorp/terraform/helper/validation"
	"github.com/rubrikinc/rubrik-sdk-for-go/rubrikcdm"
)

func resourceRubrikBootstrap() *schema.Resource {
	return &schema.Resource{
		Create: resourceRubrikBootstrapCreate,
		Read:   resourceRubrikBootstrapRead,
		Update: resourceRubrikBootstrapUpdate,
		Delete: resourceRubrikBootstrapDelete,

		Schema: map[string]*schema.Schema{
			"cluster_name": &schema.Schema{
				Type:        schema.TypeString,
				Required:    true,
				Description: "The name of the AWS S3 bucket you wish to use as an archive target.",
			},
			"admin_email": &schema.Schema{
				Type:        schema.TypeString,
				Required:    true,
				Description: "The access key of a AWS account with the required permissions.",
			},
			"admin_password": &schema.Schema{
				Type:        schema.TypeString,
				Required:    true,
				Description: "The secret key of a AWS account with the required permissions.",
				Sensitive:   true,
			},
			"management_gateway": &schema.Schema{
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.SingleIP(),
				Elem:         &schema.Schema{Type: schema.TypeString},
			},
			"management_subnet_mask": &schema.Schema{
				Type:        schema.TypeString,
				Optional:    true,
				Description: "The RSA key that will be used to encrypt the archive data.",
			},
			"dns_search_domain": &schema.Schema{
				Type:        schema.TypeList,
				Optional:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "The number of seconds to wait to establish a connection the Rubrik cluster before returning a timeout error.",
			},
			"dns_name_servers": &schema.Schema{
				Type:        schema.TypeList,
				Optional:    true,
				Description: "The number of seconds to wait to establish a connection the Rubrik cluster before returning a timeout error.",
				Elem:        &schema.Schema{Type: schema.TypeString},
			},
			"ntp_servers": &schema.Schema{
				Type:        schema.TypeList,
				Optional:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "The number of seconds to wait to establish a connection the Rubrik cluster before returning a timeout error.",
			},
			"node_config": &schema.Schema{
				Type:        schema.TypeMap,
				Optional:    true,
				Default:     false,
				Description: "The number of seconds to wait to establish a connection the Rubrik cluster before returning a timeout error.",
			},
			"enable_encryption": &schema.Schema{
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     true,
				Description: "The number of seconds to wait to establish a connection the Rubrik cluster before returning a timeout error.",
			},
			"wait_for_completion": &schema.Schema{
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     true,
				Description: "The number of seconds to wait to establish a connection the Rubrik cluster before returning a timeout error.",
			},
			"timeout": &schema.Schema{
				Type:        schema.TypeInt,
				Optional:    true,
				Default:     15,
				Description: "The number of seconds to wait to establish a connection the Rubrik cluster before returning a timeout error.",
			},
		},
	}

}

func resourceRubrikBootstrapCreate(d *schema.ResourceData, meta interface{}) error {

	rubrik := meta.(*rubrikcdm.Credentials)

	_, err := rubrik.Bootstrap(d.Get("cluster_name").(string), d.Get("admin_email").(string), d.Get("admin_password").(string), d.Get("management_gateway").(string), d.Get("management_subnet_mask").(string), d.Get("dns_search_domain").([]string), d.Get("dns_name_servers").([]string), d.Get("ntp_servers").([]string), d.Get("node_config").(map[string]string), d.Get("enable_encryption").(bool), d.Get("wait_for_completion").(bool), d.Get("timeout").(int))
	if err != nil {
		return err
	}
	d.SetId(d.Get("cluster_name").(string))

	return resourceRubrikBootstrapRead(d, meta)
}

func resourceRubrikBootstrapRead(d *schema.ResourceData, meta interface{}) error {

	rubrik := meta.(*rubrikcdm.Credentials)

	bootstrapStatus, err := rubrik.ClusterBootstrapStatus()
	if err != nil {
		return err
	}

	if bootstrapStatus {
		d.Set("cluster_name", d.Get("cluster_name").(string))
		d.Set("admin_email", d.Get("admin_email").(string))
		d.Set("admin_password", d.Get("admin_password").(string))
		d.Set("management_gateway", d.Get("management_gateway").(string))
		d.Set("management_subnet_mask", d.Get("management_subnet_mask").(string))
		d.Set("dns_search_domain", d.Get("dns_search_domain").([]string))
		d.Set("dns_name_servers", d.Get("dns_name_servers").([]string))
		d.Set("ntp_servers", d.Get("ntp_servers").([]string))
		d.Set("node_config", d.Get("node_config").(map[string]string))
		d.Set("enable_encryption", d.Get("enable_encryption").(bool))
		d.Set("wait_for_completion", d.Get("wait_for_completion").(bool))
	} else {
		d.SetId("")
	}

	return nil

}

func resourceRubrikBootstrapUpdate(d *schema.ResourceData, meta interface{}) error {
	// Once a Cluster has been bootstrapped it can not be updated through the bootstrap resource

	return resourceRubrikBootstrapRead(d, meta)
}

func resourceRubrikBootstrapDelete(d *schema.ResourceData, meta interface{}) error {
	// Once a Cluster has been bootstrapped it can not be deleted.

	return nil
}
