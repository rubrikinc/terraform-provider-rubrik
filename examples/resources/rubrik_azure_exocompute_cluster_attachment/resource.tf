resource "rubrik_azure_exocompute_cluster_attachment" "cluster" {
  cluster_name  = "my-aks-cluster"
  exocompute_id = rubrik_azure_exocompute.host.id
}
