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
		Delete: resourceRubrikClusterVersionDelete,

		Schema: map[string]*schema.Schema{
			"cluster_version": &schema.Schema{
				Type:     schema.TypeString,
				Computed: true,
			},
		},
	}

}

func resourceRubrikClusterVersionCreate(d *schema.ResourceData, meta interface{}) error {

	return resourceRubrikClusterVersionRead(d, meta)
}

func resourceRubrikClusterVersionRead(d *schema.ResourceData, meta interface{}) error {

	client := meta.(*rubrikcdm.Credentials)

	clusterVersion, err := client.ClusterVersion()
	if err != nil {
		return err
	}

	log.Printf("Cluster Version: %s", clusterVersion)

	d.SetId(clusterVersion)

	d.Set("cluster_version", clusterVersion)

	return nil
}

func resourceRubrikClusterVersionDelete(d *schema.ResourceData, m interface{}) error {
	return nil
}
