package main

import (
	"fmt"
	"log"
	"strings"

	"github.com/hashicorp/terraform/helper/schema"
	"github.com/rubrikinc/rubrik-sdk-for-go/rubrikcdm"
)

func resourceRubrikAWSNativeAccount() *schema.Resource {
	return &schema.Resource{
		Create: resourceRubrikAWSNativeAccountCreate,
		Read:   resourceRubrikAWSNativeAccountRead,
		Update: resourceRubrikAWSNativeAccountUpdate,
		Delete: resourceRubrikAWSNativeAccountDelete,

		Schema: map[string]*schema.Schema{
			"aws_account_name": &schema.Schema{
				Type:        schema.TypeString,
				Required:    true,
				Description: "The name of the AWS S3 bucket you wish to use as an archive target.",
			},
			"aws_access_key": &schema.Schema{
				Type:        schema.TypeString,
				Required:    true,
				Description: "The access key of a AWS account with the required permissions.",
			},
			"aws_secret_key": &schema.Schema{
				Type:        schema.TypeString,
				Required:    true,
				Description: "The secret key of a AWS account with the required permissions.",
				Sensitive:   true,
			},
			"aws_regions": &schema.Schema{
				Type:     schema.TypeList,
				Required: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
			"bolt_config": &schema.Schema{
				Type:        schema.TypeList,
				Optional:    true,
				Sensitive:   true,
				Description: "The RSA key that will be used to encrypt the archive data.",
				Elem:        &schema.Schema{Type: schema.TypeMap},
			},
			"delete_snapshots": &schema.Schema{
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
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

func resourceRubrikAWSNativeAccountCreate(d *schema.ResourceData, meta interface{}) error {

	log.Println("**************************************************** CREATE ****************************************************")

	rubrik := meta.(*rubrikcdm.Credentials)

	awsRegionsString := make([]string, len(d.Get("aws_regions").([]interface{})))
	for i, v := range d.Get("aws_regions").([]interface{}) {
		awsRegionsString[i] = fmt.Sprint(v)
	}

	_, err := rubrik.AddAWSNativeAccount(d.Get("aws_account_name").(string), d.Get("aws_access_key").(string), d.Get("aws_secret_key").(string), awsRegionsString, d.Get("bolt_config").([]interface{}), d.Get("timeout").(int))
	if err != nil {
		return err
	}

	log.Println("**************************************************** SETTING THE ID ****************************************************")
	d.SetId(d.Get("aws_account_name").(string))

	return resourceRubrikAWSNativeAccountRead(d, meta)
}

func resourceRubrikAWSNativeAccountRead(d *schema.ResourceData, meta interface{}) error {

	rubrik := meta.(*rubrikcdm.Credentials)

	var accountPresent = true
	aws, err := rubrik.AWSAccountSummary(d.Get("aws_account_name").(string))
	if err != nil {
		if strings.Contains(err.Error(), "AWS Native Account was not found on the Rubrik cluster") == true {
			accountPresent = false
		} else {
			return err
		}

	}

	if accountPresent == false {
		d.SetId("")
	} else {
		d.Set("aws_account_name", aws.Name)
		d.Set("aws_access_key", aws.AccessKey)
		d.Set("aws_account_name", aws.Name)
		d.Set("aws_regions", aws.Regions)
		d.Set("bolt_config", aws.RegionalBoltNetworkConfigs)

	}

	return nil

}

func resourceRubrikAWSNativeAccountUpdate(d *schema.ResourceData, meta interface{}) error {

	rubrik := meta.(*rubrikcdm.Credentials)

	config := make(map[string]interface{})
	var accountName string
	if d.HasChange("aws_account_name") {
		config["name"] = strings.ToUpper(d.Get("aws_account_name").(string))
		old, _ := d.GetChange("aws_account_name")
		accountName = old.(string)
	} else {
		accountName = d.Get("aws_account_name").(string)
	}

	if d.HasChange("aws_access_key") {
		config["accessKey"] = d.Get("aws_access_key").(string)
	}

	if d.HasChange("aws_secret_key") {
		config["secretKey"] = d.Get("aws_secret_key").(string)
	}

	if d.HasChange("aws_regions") {
		config["regions"] = d.Get("aws_regions").([]interface{})
	}

	if d.HasChange("bolt_config") {
		config["regionalBoltNetworkConfigs"] = d.Get("bolt_config").(map[string]interface{})
	}

	if len(config) == 0 {
		return resourceRubrikAWSNativeAccountRead(d, meta)
	}

	_, err := rubrik.UpdateAWSNativeAccount(accountName, config, d.Get("timeout").(int))
	if err != nil {
		return err
	}

	return resourceRubrikAWSNativeAccountRead(d, meta)
}

func resourceRubrikAWSNativeAccountDelete(d *schema.ResourceData, meta interface{}) error {

	rubrik := meta.(*rubrikcdm.Credentials)

	_, err := rubrik.RemoveAWSAccount(d.Get("aws_account_name").(string), d.Get("delete_snapshots").(bool))
	if err != nil {
		if strings.Contains(err.Error(), "AWS Native Account was not found on the Rubrik cluster") == true {
			return nil
		}
		return err

	}

	return nil
}
