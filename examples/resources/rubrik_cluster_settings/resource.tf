# Download and upgrade a cluster to a specific CDM version.
resource "rubrik_cluster_settings" "cluster" {
  cluster_id = "db34f042-79ea-48b1-bab8-c40dfbf2ab82"
  version    = "9.2.0-p1-25184"
}

# Pre-stage a CDM package without upgrading.
resource "rubrik_cluster_settings" "staged" {
  cluster_id         = "f1d3b9a4-1c2e-4a6b-9f0d-2e7c8b5a3d10"
  downloaded_version = "9.2.0-p1-25184"
}

# Toggle the cluster to FAST upgrades.
resource "rubrik_cluster_settings" "fast" {
  cluster_id   = "a2c4e6f8-0b1d-4e3f-8a5c-6d7e9f0a1b2c"
  upgrade_mode = "FAST"
}
