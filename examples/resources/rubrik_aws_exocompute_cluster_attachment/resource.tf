resource "rubrik_aws_exocompute_cluster_attachment" "attachment" {
  cluster_name  = "my-eks-cluster"
  exocompute_id = rubrik_aws_exocompute.host.id
}
