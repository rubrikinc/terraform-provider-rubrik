data "rubrik_gcp_project" "project" {
  name = "my-project"
}

ephemeral "google_service_account_id_token" "pcr" {
  target_service_account = "exocompute-pull@my-project.iam.gserviceaccount.com"
  target_audience        = "https://rubrik.com/exocompute/pcr/gcp"
  include_email          = true
}

resource "rubrik_gcp_private_container_registry" "registry" {
  cloud_account_id                = data.rubrik_gcp_project.project.id
  service_account_email           = "exocompute-pull@my-project.iam.gserviceaccount.com"
  service_account_ownership_token = ephemeral.google_service_account_id_token.pcr.id_token
  url                             = "us-docker.pkg.dev/my-project/rubrik-exocompute"
}
