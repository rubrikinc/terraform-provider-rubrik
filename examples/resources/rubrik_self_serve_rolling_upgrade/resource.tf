# Enable self-serve rolling upgrade account-wide. Only one instance of this
# resource is meaningful per RSC tenant.
resource "rubrik_self_serve_rolling_upgrade" "account" {
  enabled = true
}
