package main

import (
	"log"

	"github.com/hashicorp/terraform/helper/schema"
	"github.com/rubrikinc/rubrik-sdk-for-go/rubrikcdm"
)

func resourceRubrikClusterVersion() *schema.Resource {
	return &schema.Resource{
		Create: resourceRubrikClusterVersionCreate,
		Read:   resourceRubrikClusterVersionRead,
		Update: resourceRubrikClusterVersionUpdate,
		Delete: resourceRubrikClusterVersionDelete,

		Schema: map[string]*schema.Schema{
			"address": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
			},
		},
	}
}

func resourceRubrikClusterVersionCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*rubrikcdm.Credentials)

	clusterVersion := client.ClusterVersion()

	log.Printf("Cluster Version: %s", clusterVersion)

	return resourceRubrikClusterVersionRead(d, meta)
}

func resourceRubrikClusterVersionRead(d *schema.ResourceData, meta interface{}) error {

	return nil
}

func resourceRubrikClusterVersionUpdate(d *schema.ResourceData, meta interface{}) error {

	return resourceRubrikClusterVersionRead(d, meta)
}

func resourceRubrikClusterVersionDelete(d *schema.ResourceData, m interface{}) error {
	return nil
}
