# Rubrik Provider for Terraform

- Website: https://www.terraform.io
- [![Gitter chat](https://badges.gitter.im/hashicorp-terraform/Lobby.png)](https://gitter.im/hashicorp-terraform/Lobby)
- Mailing list: [Google Groups](http://groups.google.com/group/terraform-tool)

<img src="https://cdn.rawgit.com/hashicorp/terraform-website/master/content/source/assets/images/logo-hashicorp.svg" width="600px">


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

## Quick Start

[Quick Start Guide](https://github.com/rubrikinc/rubrik-provider-for-terraform/blob/master/docs/quickstart.md)

## Documentation

[Provider Documentation](https://rubrik.gitbook.io/terraform-provider-for-rubrik/)

## Example 

```hcl
provider "rubrik" {}

resource "rubrik_configure_timezone" "LA-Timezone" {
  timezone = "America/Los_Angeles"
}
```
