data "rubrik_cluster_versions" "cluster" {
  cluster_id = "db34f042-79ea-48b1-bab8-c40dfbf2ab82"
}

# Track the release recommended by RSC for this cluster.
resource "rubrik_cluster_settings" "cluster" {
  cluster_id = data.rubrik_cluster_versions.cluster.cluster_id
  version    = data.rubrik_cluster_versions.cluster.recommended_version
}

output "available_versions" {
  value = data.rubrik_cluster_versions.cluster.available_versions
}

output "latest_version" {
  value = data.rubrik_cluster_versions.cluster.latest_version
}
