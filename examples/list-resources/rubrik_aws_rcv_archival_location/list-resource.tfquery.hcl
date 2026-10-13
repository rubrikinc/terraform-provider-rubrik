list "rubrik_aws_rcv_archival_location" "all" {
  provider = rubrik
}

list "rubrik_aws_rcv_archival_location" "by_name" {
  provider = rubrik

  config {
    name = "us-east-1-archive"
  }
}
