# Service account from the current environment.
provider "rubrik" {
}

# Service account from the content of a service account file.
provider "rubrik" {
  credentials = <<-EOS
    {
      "client_id": "client|...",
      "client_secret": "...",
      "name": "dummy-service-account",
      "access_token_uri": "https://account.my.rubrik.com/api/client_token"
    }
    EOS
}

provider "rubrik" {
  credentials = "<content of service-account-credentials.json>"
}

# Service account from file.
provider "rubrik" {
  credentials = "/path/to/service-account-credentials.json"
}

# Local user account.
provider "rubrik" {
  credentials = "my-account"
}
