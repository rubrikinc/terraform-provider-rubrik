data "rubrik_cluster_settings" "cluster" {
  cluster_id = "db34f042-79ea-48b1-bab8-c40dfbf2ab82"
}

output "installed_version" {
  value = data.rubrik_cluster_settings.cluster.version
}

output "upgrade_status" {
  value = data.rubrik_cluster_settings.cluster.upgrade_status_v2.rsc_cluster_upgrade_status
}
