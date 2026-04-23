data "rubrik_role" "auditor" {
  name = "Compliance Auditor Role"
}

resource "rubrik_user" "auditor" {
  email = "auditor@example.org"

  role_ids = [
    data.rubrik_role.auditor.id
  ]
}
