# Terraform本体とプロバイダのバージョン制約をまとめます。
# boki3はfeedflowの既存EC2へ同居するため、AWSリソースは一切作成しません。
# 管理対象はCloudflare (DNS / Origin CA証明書 / Access) と、既存EC2へのデプロイだけです。

terraform {
  required_version = ">= 1.6.0"

  required_providers {
    cloudflare = {
      source  = "cloudflare/cloudflare"
      version = "~> 4.40"
    }
    tls = {
      source  = "hashicorp/tls"
      version = "~> 4.0"
    }
    null = {
      source  = "hashicorp/null"
      version = "~> 3.2"
    }
  }
}

# CloudflareプロバイダですDNSとAccessとOrigin CA証明書の発行をAPIトークンで操作します。
# トークンにはSSL and Certificatesの編集権限を含めます (feedflowと同じトークンを使えます)。
provider "cloudflare" {
  api_token = var.cloudflare_api_token
}
