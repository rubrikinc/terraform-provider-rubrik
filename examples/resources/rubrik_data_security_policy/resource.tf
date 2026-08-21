# Policy matching highly sensitive documents in unprotected objects. A
# single object condition group is the simplest useful policy.
resource "rubrik_data_security_policy" "overexposed_sensitive_data" {
  name        = "Overexposed Sensitive Data"
  description = "Highly sensitive documents without backup protection"
  category    = "OVEREXPOSED"
  severity    = "CRITICAL"

  object_filter {
    op = "AND"

    condition {
      filter_type  = "SECURITY_DOCUMENT_SENSITIVITY"
      values       = ["HIGH", "MEDIUM"]
      relationship = "IS"
    }

    condition {
      filter_type  = "SECURITY_SNAPPABLE_BACKUP"
      values       = ["Unprotected"]
      relationship = "IS"
    }
  }
}

# Policy combining object and identity conditions. The two blocks are
# always joined by AND, while the conditions inside each block are joined
# by the block's op field.
resource "rubrik_data_security_policy" "exposed_to_service_accounts" {
  name        = "Sensitive Data Exposed To Service Accounts"
  description = "Sensitive documents reachable by service accounts"
  category    = "OVEREXPOSED"
  severity    = "HIGH"

  object_filter {
    op = "OR"

    condition {
      filter_type  = "SECURITY_DOCUMENT_SENSITIVITY"
      values       = ["HIGH"]
      relationship = "IS"
    }

    condition {
      filter_type  = "SECURITY_SNAPPABLE_ENCRYPTION"
      values       = ["ENCRYPTION_DISABLED"]
      relationship = "IS"
    }
  }

  identity_filter {
    op = "AND"

    condition {
      filter_type  = "SECURITY_IDENTITY_NAME"
      values       = ["svc-"]
      relationship = "CONTAINS"
    }
  }
}

# Policy raising a violation only once enough documents match. The
# threshold filter is typically a hit count condition.
resource "rubrik_data_security_policy" "bulk_sensitive_data" {
  name        = "Bulk Sensitive Data"
  description = "Objects holding more than 100 highly sensitive documents"
  category    = "MISPLACED"
  severity    = "MEDIUM"
  enabled     = false

  object_filter {
    condition {
      filter_type  = "SECURITY_DOCUMENT_SENSITIVITY"
      values       = ["HIGH"]
      relationship = "IS"
    }
  }

  threshold_filter {
    filter_type  = "SECURITY_DOCUMENT_HIT_COUNT"
    values       = ["100"]
    relationship = "GREATER_THAN"
  }
}
