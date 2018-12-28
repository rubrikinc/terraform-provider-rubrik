package main

import (
	"log"

	"github.com/hashicorp/terraform/helper/schema"
	"github.com/hashicorp/terraform/helper/validation"
	"github.com/rubrikinc/rubrik-sdk-for-go/rubrikcdm"
)

func resourceRubrikConfigureTimezone() *schema.Resource {
	return &schema.Resource{
		Create: resourceRubrikConfigureTimezoneCreate,
		Read:   resourceRubrikConfigureTimezoneRead,
		Update: resourceRubrikConfigureTimezoneUpdate,
		Delete: resourceRubrikConfigureTimezoneDelete,

		Schema: map[string]*schema.Schema{
			"timezone": &schema.Schema{
				Type:     schema.TypeString,
				Required: true,
				ValidateFunc: validation.StringInSlice([]string{
					"America/Anchorage",
					"America/Araguaina",
					"America/Barbados",
					"America/Chicago",
					"America/Denver",
					"America/Los_Angeles",
					"America/Mexico_City",
					"America/New_York",
					"America/Noronha",
					"America/Phoenix",
					"America/Toronto",
					"America/Vancouver",
					"Asia/Bangkok",
					"Asia/Dhaka",
					"Asia/Dubai",
					"Asia/Hong_Kong",
					"Asia/Karachi",
					"Asia/Kathmandu",
					"Asia/Kolkata",
					"Asia/Magadan",
					"Asia/Singapore",
					"Asia/Tokyo",
					"Atlantic/Cape_Verde",
					"Australia/Perth",
					"Australia/Sydney",
					"Europe/Amsterdam",
					"Europe/Athens",
					"Europe/London",
					"Europe/Moscow",
					"Pacific/Auckland",
					"Pacific/Honolulu",
					"Pacific/Midway",
					"UTC",
				}, true),
			},
			"timeout": &schema.Schema{
				Type:     schema.TypeInt,
				Optional: true,
				Default:  15,
			},
		},
	}

}

func resourceRubrikConfigureTimezoneCreate(d *schema.ResourceData, meta interface{}) error {

	rubrik := meta.(*rubrikcdm.Credentials)

	rubrik.ConfigureTimezone(d.Get("timezone").(string))

	return resourceRubrikConfigureTimezoneRead(d, meta)
}

func resourceRubrikConfigureTimezoneRead(d *schema.ResourceData, meta interface{}) error {

	rubrik := meta.(*rubrikcdm.Credentials)

	log.Println("[INFO] Determining the current Rubrik cluster timezone.")
	clusterSummary, err := rubrik.Get("v1", "/cluster/me")
	if err != nil {
		return err
	}

	currentTimezone := clusterSummary.(map[string]interface{})["timezone"].(map[string]interface{})["timezone"]

	d.SetId("rubrik-cluster-timezone")

	d.Set("timezone", currentTimezone)

	return nil

}

func resourceRubrikConfigureTimezoneUpdate(d *schema.ResourceData, meta interface{}) error {

	rubrik := meta.(*rubrikcdm.Credentials)

	rubrik.ConfigureTimezone(d.Get("timezone").(string))

	return nil
}

func resourceRubrikConfigureTimezoneDelete(d *schema.ResourceData, m interface{}) error {
	// Cluster Timezone is a requirement for the Rubrik cluster and can not be "deleted"
	return nil
}
