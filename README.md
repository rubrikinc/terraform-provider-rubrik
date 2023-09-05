# Rubrik Provider for Terraform

- Website: https://www.terraform.io
- [![Gitter chat](https://badges.gitter.im/hashicorp-terraform/Lobby.png)](https://gitter.im/hashicorp-terraform/Lobby)
- Mailing list: [Google Groups](http://groups.google.com/group/terraform-tool)

<img src="https://cdn.rawgit.com/hashicorp/terraform-website/master/content/source/assets/images/logo-hashicorp.svg" width="600px">


# :hammer: Installation

Requirements: Terraform has been successfully [installed](https://learn.hashicorp.com/terraform/getting-started/install.html).


1. Download the latest compiled binary from [GitHub releases](../../releases).
   ```
   macOS Intel: terraform-provider-rubrik-darwin-amd64
   macOS Apple: terraform-provider-rubrik-darwin-arm64
   Linux: terraform-provider-rubrik-linux-amd64
   Windows: terraform-provider-rubrik-windows-amd64.exe
   ```

2. Move the Rubrik provider into the correct Terraform plugin directory
     
   **For Terraform 0.12 and earlier:**
   
   ````
   macOS Intel: ~/.terraform.d/plugins/darwin_amd64
   macOS Apple: ~/.terraform.d/plugins/darwin_arm64
   Linux: ~/.terraform.d/plugins/linux_amd64
   Windows: %APPDATA%\terraform.d\plugins\windows_amd64
   ````
   Note: _You may need to create the plugins directory._

   **For Terraform 0.13 and later:**

   ````
   macOS Intel: cp terraform-provider-rubrik-darwin-amd64 ~/.terraform.d/plugins/rubrikinc/rubrik/rubrik/<release_version>/darwin_amd64/terraform-provider-rubrik
   macOS Apple: cp terraform-provider-rubrik-darwin-arm64 ~/.terraform.d/plugins/rubrikinc/rubrik/rubrik/<release_version>/darwin_arm64/terraform-provider-rubrik
   Linux: cp terraform-provider-rubrik-linux-amd64 ~/.terraform.d/plugins/rubrikinc/rubrik/rubrik/<release_version>/linux_amd64/terraform-provider-rubrik
   Windows: copy terraform-provider-rubrik-windows-amd64.exe %APPDATA%\terraform.d\plugins\rubrikinc\rubrik\rubrik\<release_version>\windows_amd64\terraform-provider-rubrik.exe
   ````
   Note: _You may need to create the containing directory structure._

   Note: _Replace <release_version> with the release number of the provider as found in [GitHub releases](../../releases). Example: 2.2.0_

   Note: _`terraform-provider-rubrik` and `terraform-provider-rubrik.exe` are file names not directories._


3. For MacOS and Linux only, make the `terraform-provider-rubrik` file executable.

   ````
   macOS chmod 755 ~/.terraform.d/plugins/rubrikinc/rubrik/rubrik/<release_version>/darwin_amd64/terraform-provider-rubrik
   Linux: chmod 755 ~/.terraform.d/plugins/rubrikinc/rubrik/rubrik/<release_version>/linux_amd64/terraform-provider-rubrik
   ````

4. Run `terraform init` in the directory that contains your Terraform configuration file (`main.tf`)

# :blue_book: Documentation

Here are some resources to get you started! If you find any challenges from this project are not properly documented or are unclear, please [raise an issue](../../issues/new/choose) and let us know! This is a fun, safe environment - don't worry if you're a GitHub newbie! :heart:

* [Quick Start Guide](docs/quick-start.md)
* [Rubrik Provider for Terraform Documentation](https://rubrik.gitbook.io/terraform-provider-for-rubrik/)
* [Rubrik API Documentation](https://github.com/rubrikinc/api-documentation)
* [VIDEO: Getting Started with the Rubrik Provider for Terraform](https://www.youtube.com/watch?v=kV1xiP1tHY0)
* [BLOG: Using Terraform with Rubrik Just Got Easier!](https://www.rubrik.com/blog/rubrik-provider-terraform/)

## :mag: Example 

```hcl
provider "rubrik" {}

resource "rubrik_configure_timezone" "LA-Timezone" {
  timezone = "America/Los_Angeles"
}
```

# :muscle: How You Can Help

We glady welcome contributions from the community. From updating the documentation to adding more functions for Terraform, all ideas are welcome. Thank you in advance for all of your issues, pull requests, and comments! :star:

* [Contributing Guide](CONTRIBUTING.md)
* [Code of Conduct](CODE_OF_CONDUCT.md)

# :pushpin: License

* [MIT License](LICENSE)

# :point_right: About Rubrik Build

We encourage all contributors to become members. We aim to grow an active, healthy community of contributors, reviewers, and code owners. Learn more in our [Welcome to the Rubrik Build Community](https://github.com/rubrikinc/welcome-to-rubrik-build) page.
