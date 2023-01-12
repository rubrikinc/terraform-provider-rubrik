# Change Log

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](http://keepachangelog.com/)
and this project adheres to [Semantic Versioning](http://semver.org/).

## Types of changes

- **Added** for new features.
- **Changed** for changes in existing functionality.
- **Deprecated** for soon-to-be removed features.
- **Removed** for now removed features.
- **Fixed** for any bug fixes.
- **Security** in case of vulnerabilities.

## 0.1.0 (Nov 22, 2019)

IMPROVEMENTS:

* CHANGELOG.md created
* Added GNUmakefile
* Added acceptance tests for `provider.go`, `data_source_rubrik_cluster_version.go`, `resource_rubrik_assign_sla.go`, `resource_rubrik_configure_timezone.go`
* Changed provider to look for upper case environment authentication variables
* Added check for lower case environment variables to compatibility with other Rubrik SDKs
* Converted existing code to utilize [Terraform plugin SDK](https://www.terraform.io/docs/extend/plugin-sdk.html)
* Added `go.mod` to support versioned [Go Modules](https://github.com/golang/go/wiki/Modules)

# 1.0.1

* Changed to include Rubrik Go SDK v1.0.1. 
* Fixed This fixes issues with the bootstrap resource.

## 1.0.2

* Changed Improved handling of potential TCP connectivity issues that may occur when bootstrapping a Cloud Cluster whose services are still coming online

## 1.0.3

* Added a new aws_export_ec2 provider.

## 1.0.4

* Changed - Specify a date_time value in the rubrik_aws_export_ec2 resource
* Changed rubrikcdm Go SDK to latest version

## 2.1.0

* Added support for CCES on AWS and Azure. - Note this is a breaking change for CDM versions older than v5.0.
* Changed to work with Terraform v1.2.2
* Changed to bootstrap CDM v5.0 and higher clusters with secure NTP. - Note this is a breaking change for CDM versions older than v5.0.
* Changed to rubrikcdm Go SDK v1.1.1