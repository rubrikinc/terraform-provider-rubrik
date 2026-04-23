# Output the IP addresses and version used by the RSC deployment.
data "rubrik_deployment" "deployment" {}

output "ip_addresses" {
  value = data.rubrik_deployment.deployment.ip_addresses
}

output "version" {
  value = data.rubrik_deployment.deployment.version
}
