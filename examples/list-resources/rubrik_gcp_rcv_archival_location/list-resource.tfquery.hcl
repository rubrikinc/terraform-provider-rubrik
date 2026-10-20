list "rubrik_gcp_rcv_archival_location" "all" {
  provider = rubrik
}

list "rubrik_gcp_rcv_archival_location" "by_name" {
  provider = rubrik

  config {
    name = "us-west1-archive"
  }
}
