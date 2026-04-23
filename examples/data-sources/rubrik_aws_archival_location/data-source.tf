# Using the ID.
data "rubrik_aws_archival_location" "location" {
  id = "db34f042-79ea-48b1-bab8-c40dfbf2ab82"
}

# Using the name.
data "rubrik_aws_archival_location" "location" {
  name = "my-archival-location"
}
