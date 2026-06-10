# Terraform Provider for Rubrik

For documentation of all resources and their parameters head over to the
[Terraform Registry Docs](https://registry.terraform.io/providers/rubrikinc/rubrik/latest/docs).

## Use the Official Build

To use the official version of the provider built by Rubrik and published to the Terraform Registry, use the following
snippet at the top of your Terraform configuration:

```terraform
terraform {
  required_providers {
    rubrik = {
      source = "rubrikinc/rubrik"
    }
  }
}
```

This will pull down the latest version of the provider from the Terraform Registry. Terraform will also validate the
authenticity of the provider using cryptographically signed checksums.

### Environment Variables

The following environmental variables can be used to override the default behaviour of the provider:

* *RUBRIK_LOGLEVEL* — Overrides the log level of the provider. Valid log levels are: *FATAL*, *ERROR*,
  *WARN*, *INFO*, *DEBUG*, *TRACE* and *OFF*. The default log level of the provider is *WARN*.
* *RUBRIK_TOKEN_CACHE* — Overrides whether the token cache should be used or not. By default, the token
  cache is used.
* *RUBRIK_TOKEN_CACHE_DIR* — Overrides the directory where cached authentication tokens are stored. By default,
  the OS default directory for temporary files is used.
* *RUBRIK_TOKEN_CACHE_SECRET* — Overrides the secret used as input when generating an encryption key for the
  authentication token.

### Terraform Logging Support

The provider supports Terraform's native logging system (tflog) for improved debugging and troubleshooting. This
provides structured logging with better integration into Terraform's logging infrastructure.

#### Terraform Logging Environment Variables

* *TF_LOG_PROVIDER_RUBRIK* — Controls the log level for the Terraform provider itself. Valid log levels are: *TRACE*,
  *DEBUG*, *INFO*, *WARN*, *ERROR*, and *OFF*. This variable follows Terraform's standard logging conventions.
* *TF_LOG_PROVIDER_RUBRIK_API* — Controls the log level specifically for API calls made by the provider to RSC. This
  allows you to separately control the verbosity of API-related logging.

#### Usage Examples

```shell
# Enable DEBUG logging for the provider, including API calls
export TF_LOG_PROVIDER_RUBRIK=DEBUG

# Enable TRACE logging for API calls only
export TF_LOG_PROVIDER_RUBRIK_API=TRACE

# Enable both provider and API logging at different levels
export TF_LOG_PROVIDER_RUBRIK=INFO
export TF_LOG_PROVIDER_RUBRIK_API=DEBUG

# Direct provider logs to a specific file
export TF_LOG_PROVIDER_PATH=./rubrik-provider.log
```

### Provider Credentials

The provider authenticates with RSC using a service account. For documentation on how to create a service account using
RSC see the [Rubrik Support Portal](http://support.rubrik.com).

To use a service account with the provider first download the service account credentials as a JSON file from the
RSC User Management UI page. Next, configure the provider to use the downloaded credentials file in the
Terraform configuration:

```terraform
terraform {
  required_providers {
    rubrik = {
      source = "rubrikinc/rubrik"
    }
  }
}

provider "rubrik" {
  credentials = "/path/to/credentials.json"
}
```

#### Environment Variables for Service Accounts

When using a service account the following environmental variables can be used to override the default service account
behaviour:

* *RUBRIK_SERVICEACCOUNT_CREDENTIALS* — Overrides the content of the service account credentials file.
* *RUBRIK_SERVICEACCOUNT_FILE* — Overrides the name and path of the service account credentials file.
* *RUBRIK_SERVICEACCOUNT_NAME* — Overrides the name of the service account.
* *RUBRIK_SERVICEACCOUNT_CLIENTID* — Overrides the client id of the service account.
* *RUBRIK_SERVICEACCOUNT_CLIENTSECRET* — Overrides the client secret of the service account.
* *RUBRIK_SERVICEACCOUNT_ACCESSTOKENURI* — Overrides the service account access token URI. When using a
  service account the RSC API URL is derived from this URI.

## Use Your Own Build

### Build

To build the provider for your machines OS and hardware architecture run the following command in the root of the
repository:

```shell
go build
```

After the build finishes you should have a binary named `terraform-provider-rubrik` in the root of the repository.

### Install

To install the newly built provider on your machine you first need to create the directory where Terraform looks for
local providers:

```shell
mkdir -p ~/.terraform.d/plugins
```

Next you need to copy the provider binary into a subdirectory of `~/.terraform.d/plugins`, the exact subdirectory
depends on your machines OS and hardware architecture. For Linux/AMD64 the subdirectory would be
`terraform.rubrik.com/rubrikinc/rubrik/0.0.1/linux_amd64`, where `0.0.1` is the version of the provider binary. This
can either be `0.0.1` or the release tag closest to the commit you built. For the release tag `v0.1.0` you would use
`0.1.0`. So the commands for copying a build of the `v0.1.0` release tag on a Linux/AMD64 machine would be:

```shell
mkdir ~/.terraform.d/plugins/terraform.rubrik.com/rubrikinc/rubrik/0.1.0/linux_amd64
cp terraform-provider-rubrik ~/.terraform.d/plugins/terraform.rubrik.com/rubrikinc/rubrik/0.1.0/linux_amd64
```

After the above commands the directory structure under `~/.terraform.d` would be:

```shell
.terraform.d/
└── plugins/
    └── terraform.rubrik.com/
        └── rubrikinc/
            └── rubrik/
                └── 0.1.0/
                    └── linux_amd64/
                        └── terraform-provider-rubrik
```

### Use

To use the local provider in a Terraform configuration use the following snippet at the top of the file:

```terraform
terraform {
  required_providers {
    rubrik = {
      source  = "terraform.rubrik.com/rubrikinc/rubrik"
    }
  }
}
```
