package rubrikcdm

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"github.com/rubrikinc/rubrik-sdk-for-go/rubrikcdm"
)

func resourceRubrikBootstrapCcesAzure() *schema.Resource {
	return &schema.Resource{
		Create: resourceRubrikBootstrapCcesAzureCreate,
		Read:   resourceRubrikBootstrapCcesAzureRead,
		Update: resourceRubrikBootstrapCcesAzureUpdate,
		Delete: resourceRubrikBootstrapCcesAzureDelete,

		Schema: map[string]*schema.Schema{
			"cluster_name": &schema.Schema{
				Type:        schema.TypeString,
				Required:    true,
				Description: "Unique name to assign to the Rubrik cluster.",
			},
			"admin_email": &schema.Schema{
				Type:        schema.TypeString,
				Required:    true,
				Description: "The Rubrik cluster sends messages for the admin account to this email address.",
			},
			"admin_password": &schema.Schema{
				Type:        schema.TypeString,
				Required:    true,
				Sensitive:   true,
				Description: "Password for the admin account.",
			},
			"management_gateway": &schema.Schema{
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.IsIPAddress,
				Description:  "IP address assigned to the management network gateway",
			},
			"management_subnet_mask": &schema.Schema{
				Type:        schema.TypeString,
				Required:    true,
				Description: "Subnet mask assigned to the management network.",
			},
			"dns_search_domain": &schema.Schema{
				Type:        schema.TypeList,
				Required:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "The search domain that the DNS Service will use to resolve hostnames that are not fully qualified.",
			},
			"dns_name_servers": &schema.Schema{
				Type:        schema.TypeList,
				Required:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "IPv4 addresses of DNS servers.",
			},
			"ntp_server1_name": &schema.Schema{
				Type:        schema.TypeString,
				Required:    true,
				Description: "IP address for NTP server #1.",
			},
			"ntp_server1_key_id": &schema.Schema{
				Type:        schema.TypeInt,
				Optional:    true,
				Description: "Key id number for NTP server #1 (typically this is 0)",
			},
			"ntp_server1_key": &schema.Schema{
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Symmetric key material for NTP server #1.",
			},
			"ntp_server1_key_type": &schema.Schema{
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Symmetric key type for NTP server #1.",
			},
			"ntp_server2_name": &schema.Schema{
				Type:        schema.TypeString,
				Required:    true,
				Description: "IP address for NTP server #2.",
			},
			"ntp_server2_key_id": &schema.Schema{
				Type:        schema.TypeInt,
				Optional:    true,
				Description: "Key id number for NTP server #2 (typically this is 1)",
			},
			"ntp_server2_key": &schema.Schema{
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Symmetric key material for NTP server #2.",
			},
			"ntp_server2_key_type": &schema.Schema{
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Symmetric key type for NTP server #2.",
			},
			"node_config": &schema.Schema{
				Type:        schema.TypeMap,
				Required:    true,
				Description: "The Node Name and IP formatted as a map.",
			},
			"enable_encryption": &schema.Schema{
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     true,
				Description: "Enable software data encryption at rest. When bootstrapping a Cloud Cluster this value needs to be False.",
			},
			"connection_string": &schema.Schema{
				Type:        schema.TypeString,
				Required:    true,
				Description: "The connection string for the Azure storage account where CCES will store its data.",
			},
			"container_name": &schema.Schema{
				Type:        schema.TypeString,
				Required:    true,
				Description: "The name of the container in the Azure storage account where CCES will store its data.",
			},
			"enable_immutability": &schema.Schema{
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Flag to determine if versioning will be used on the Azure Blob storage to enable immutability.",
			},
			"wait_for_completion": &schema.Schema{
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     true,
				Description: "Flag to determine if the function should wait for the bootstrap process to complete.",
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

func resourceRubrikBootstrapCcesAzureCreate(d *schema.ResourceData, meta interface{}) error {

	// Convert interface{} list and maps to string
	dnsSearchDomain := make([]string, len(d.Get("dns_search_domain").([]interface{})))
	for i, v := range d.Get("dns_search_domain").([]interface{}) {
		dnsSearchDomain[i] = fmt.Sprint(v)
	}

	dnsNameServers := make([]string, len(d.Get("dns_name_servers").([]interface{})))
	for i, v := range d.Get("dns_name_servers").([]interface{}) {
		dnsNameServers[i] = fmt.Sprint(v)
	}

	ntpServers := map[string]interface{}{}
	ntpServers["ntpServer1"] = map[string]interface{}{}
	ntpServers["ntpServer1"].(map[string]interface{})["IP"] = d.Get("ntp_server1_name").(string)
	ntpServers["ntpServer1"].(map[string]interface{})["key"] = d.Get("ntp_server1_key").(string)
	ntpServers["ntpServer1"].(map[string]interface{})["keyId"] = d.Get("ntp_server1_key_id").(int)
	ntpServers["ntpServer1"].(map[string]interface{})["keyType"] = d.Get("ntp_server1_key_type").(string)
	ntpServers["ntpServer2"] = map[string]interface{}{}
	ntpServers["ntpServer2"].(map[string]interface{})["IP"] = d.Get("ntp_server2_name").(string)
	ntpServers["ntpServer2"].(map[string]interface{})["key"] = d.Get("ntp_server2_key").(string)
	ntpServers["ntpServer2"].(map[string]interface{})["keyId"] = d.Get("ntp_server2_key_id").(int)
	ntpServers["ntpServer2"].(map[string]interface{})["keyType"] = d.Get("ntp_server2_key_type").(string)

	nodeConfig := make(map[string]string)
	for key, value := range d.Get("node_config").(map[string]interface{}) {
		strKey := fmt.Sprintf("%v", key)
		strValue := fmt.Sprintf("%v", value)
		nodeConfig[strKey] = strValue
	}

	rubrik := meta.(*rubrikcdm.Credentials)
	_, err := rubrik.BootstrapCcesAzure(d.Get("cluster_name").(string), d.Get("admin_email").(string), d.Get("admin_password").(string), d.Get("management_gateway").(string), d.Get("management_subnet_mask").(string), dnsSearchDomain, dnsNameServers, ntpServers, nodeConfig, d.Get("enable_encryption").(bool), d.Get("connection_string").(string), d.Get("container_name").(string), d.Get("enable_immutability").(bool), d.Get("wait_for_completion").(bool), d.Get("timeout").(int))
	if err != nil {
		return err
	}

	d.SetId(d.Get("cluster_name").(string))

	return resourceRubrikBootstrapCcesAzureRead(d, meta)
}

func resourceRubrikBootstrapCcesAzureRead(d *schema.ResourceData, meta interface{}) error {

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
		d.Set("dns_search_domain", d.Get("dns_search_domain").([]interface{}))
		d.Set("dns_name_servers", d.Get("dns_name_servers").([]interface{}))
		d.Set("ntp_server1_name", d.Get("ntp_server1_name").(string))
		d.Set("ntp_server1_key_id", d.Get("ntp_server1_key_id").(int))
		d.Set("ntp_server1_key", d.Get("ntp_server1_key").(string))
		d.Set("ntp_server1_key_type", d.Get("ntp_server1_key_type").(string))
		d.Set("ntp_server2_name", d.Get("ntp_server2_name").(string))
		d.Set("ntp_server2_key_id", d.Get("ntp_server2_key_id").(int))
		d.Set("ntp_server2_key", d.Get("ntp_server2_key").(string))
		d.Set("ntp_server2_key_type", d.Get("ntp_server2_key_type").(string))
		d.Set("node_config", d.Get("node_config").(map[string]interface{}))
		d.Set("enable_encryption", d.Get("enable_encryption").(bool))
		d.Set("connection_string", d.Get("connection_string").(string))
		d.Set("container_name", d.Get("container_name").(string))
		d.Set("enable_immutability", d.Get("enable_immutability").(bool))
		d.Set("wait_for_completion", d.Get("wait_for_completion").(bool))
	} else {
		d.SetId("")
	}

	return nil

}

func resourceRubrikBootstrapCcesAzureUpdate(d *schema.ResourceData, meta interface{}) error {
	// Once a Cluster has been bootstrapped it can not be updated through the bootstrap resource

	return resourceRubrikBootstrapRead(d, meta)
}

func resourceRubrikBootstrapCcesAzureDelete(d *schema.ResourceData, meta interface{}) error {
	// Once a Cluster has been bootstrapped it can not be deleted.

	return nil
}
