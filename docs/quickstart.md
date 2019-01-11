
# Quick Start Guide: Terraform Provider for Rubrik

## Introduction to the Terraform Provider for Rubrik

Rubrik's API first architecture enables organizations to embrace and integrate Rubrik functionality into their existing automation processes. While Rubrik APIs can be consumed natively, companies are at various stages in their automation journey with different levels of automation knowledge on staff. The Rubrik Terraform Provder transform the Rubrik RESTful API functionality into easy to consume Terraform configuration whichs eliminates the need to understand how to consume raw Rubrik APIs extends upon one of Rubrik's main design centers - simplicity

## Authentication

The Rubrik provider offers a flexible means of providing credentials for
authentication. The following methods are supported, in this order, and
explained below:

- Environment variables
- Static credentials

### Environment variables

Storing credentials in environment variables is a more secure process than storing them in your source code, and it ensures that your credentials are not accidentally shared if your code is uploaded to an internal or public version control system such as GitHub. 

* **rubrik_cdm_node_ip** (Contains the IP/FQDN of a Rubrik node)
* **rubrik_cdm_username** (Contains a username with configured access to the Rubrik cluster)
* **rubrik_cdm_password** (Contains the password for the above user).



```hcl
provider "rubrik" {}
```


#### Setting Environment Variables in Microsoft Windows

For Microsoft Windows-based operating systems the environment variables can be set utilizing the setx command as follows:

```
setx rubrik_cdm_node_ip "192.168.0.100"
setx rubrik_cdm_username "user@domain.com"
setx rubrik_cdm_password "SecretPassword"
```

Run set without any other parameters to view current environment variables. Using setx saves the environment variables permanently, and the variables defined in the current shell will not be available until a new shell is opened. Using set instead of setx will define variables in the current shell session, but they will not be saved between sessions.

#### Setting Environment Variables in macOS and \*nix

For macOS and \*nix based operating systems the environment variables can be set utilizing the export command as follows:

```
export rubrik_cdm_node_ip=192.168.0.100
export rubrik_cdm_username=user@domain.com
export rubrik_cdm_password=SecretPassword
```

Run export without any other parameters to view current environment variables. In order for the environment variables to persist across terminal sessions, add the above three export commands to the `~\.bash_profile` or `~\.profile` file.

### Static credentials 

Static credentials can be provided by adding an `node_ip`, `username` and `password` in-line in the
Rubrik provider block:

Usage:

```hcl
provider "rubrik" {
  node_ip     = "10.255.41.201"
  username    = "admin"
  password    = "RubrikTFDemo2019"
}
```

## Terraform Provider for Rubrik Quickstart


## Installation

Requirements: Terraform has been successfully [installed](https://learn.hashicorp.com/terraform/getting-started/install.html).

1. Download the latest compiled binary from [GitHub releases](https://github.com/rubrikinc/rubrik-provider-for-terraform/releases).
   ```
   macOS: terraform-provider-rubrik-darwin-amd64
   Linux: terraform-provider-rubrik-linux-amd64
   Windows: terraform-provider-rubrik-windows-amd64.exe
   ```

2. Move the Rubrik provder into the correct Terraform plugin directory
   
   ```
   macOS: ~/.terraform.d/plugins/darwin_amd64
   Linux: ~/.terraform.d/plugins/linux_amd64
   Windows: %APPDATA%\terraform.d\plugins\windows_amd64
   ```
   
   _You may need to manually create the `plugin` directory._

3. Rename the the Rubrik provder to `terraform-provider-rubrik`

4. Run `terraform init` in the directory that contains your Terraform configuration fiile (`main.tf`)

### Sample Syntax - Cluster Timezone Configuration

```hcl
provider "rubrik" {}

resource "rubrik_configure_timezone" "LA-Timezone" {
  timezone = "America/Los_Angeles"
}
```

### Terraform Provider for Rubrik Documentation

This guide acts only as a quick start to get up and running with the Terraform Provider for Rubrik. For detailed information on all of the functions and features included see the complete [Terraform Provider for Rubrik documentation](https://rubrik.gitbook.io/terraform-provider-for-rubrik/).

