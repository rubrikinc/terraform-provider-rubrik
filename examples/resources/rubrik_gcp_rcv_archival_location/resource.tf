resource "rubrik_gcp_rcv_archival_location" "archive" {
  name   = "us-west1-archive"
  region = "us-west1"
  tier   = "ARCHIVE"
}
