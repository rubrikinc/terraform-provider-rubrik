# Look up SLA source cluster by name.
data "rubrik_sla_source_cluster" "cluster" {
  name = "my-cluster"
}

output "cluster_id" {
  value = data.rubrik_sla_source_cluster.cluster.id
}

