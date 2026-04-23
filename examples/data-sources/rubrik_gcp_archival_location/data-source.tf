# Using the ID.
data "rubrik_gcp_archival_location" "location" {
  id = "9e90a8bb-0578-43dc-9330-57f86a9ae1e6"
}

# Using the name.
data "rubrik_gcp_archival_location" "location" {
  name = "my-archival-location"
}
