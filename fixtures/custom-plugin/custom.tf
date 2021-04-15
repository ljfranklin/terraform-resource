terraform {
  required_providers {
    custom = {
      source  = "github.com/ljfranklin/custom"
      version = ">= 999.999.999"
    }
  }
  required_version = ">= 0.13"
}

resource "tls_private_key" "example" {
  provider    = custom
  algorithm   = "ECDSA"
  ecdsa_curve = "P384"
}
