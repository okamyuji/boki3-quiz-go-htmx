# Cloudflare側のリソースをまとめます。
# DNSのAレコードはプロキシONでエッジHTTPS終端とオリジンIP秘匿を行います。
# Origin CA証明書をオリジンに置きSSLモードFull(strict)に対応します。
# Cloudflare Accessで所有者メールだけを通過させ本人限定にします。
#
# ゾーン全体の設定 (SSL strict / always_use_https など) はfeedflowのterraformが
# cloudflare_zone_settings_override で管理済みのため、ここでは重複管理しません。

# 管理対象ゾーンを名前から解決します。
data "cloudflare_zone" "this" {
  name = var.zone_name
}

# Cloudflareのエッジ送信元IP範囲を取得し、nginxのreal_ip復元に使います。
data "cloudflare_ip_ranges" "cloudflare" {}

# オリジン証明書用の秘密鍵をローカルで生成します。
resource "tls_private_key" "origin" {
  algorithm = "RSA"
  rsa_bits  = 2048
}

# Origin CAへ提出するCSRを作成します。
resource "tls_cert_request" "origin" {
  private_key_pem = tls_private_key.origin.private_key_pem

  subject {
    common_name = var.hostname
  }

  dns_names = [var.hostname]
}

# Cloudflare Origin CA証明書を発行します。CF→オリジン間でのみ有効な証明書です。
resource "cloudflare_origin_ca_certificate" "origin" {
  csr                = tls_cert_request.origin.cert_request_pem
  hostnames          = [var.hostname]
  request_type       = "origin-rsa"
  requested_validity = 5475
}

# Aレコードをfeedflowと同じEIPへ向けます。proxied=trueでCloudflareがHTTPSを終端しIPを隠します。
resource "cloudflare_record" "app" {
  zone_id = data.cloudflare_zone.this.id
  name    = var.hostname
  type    = "A"
  content = var.origin_public_ip
  proxied = true
  ttl     = 1
  comment = "boki3-quiz origin (feedflow EC2に同居)"
}

# Cloudflare Accessのアプリケーションです。指定ホスト名全体を保護します。
resource "cloudflare_zero_trust_access_application" "app" {
  account_id                = var.cloudflare_account_id
  name                      = "${var.project_name} owner only"
  domain                    = var.hostname
  type                      = "self_hosted"
  session_duration          = "720h"
  app_launcher_visible      = false
  auto_redirect_to_identity = false
}

# 所有者のメールだけを許可するポリシーです。それ以外はブロックされます。
resource "cloudflare_zero_trust_access_policy" "owner" {
  account_id     = var.cloudflare_account_id
  application_id = cloudflare_zero_trust_access_application.app.id
  name           = "owner email allow"
  precedence     = 1
  decision       = "allow"

  include {
    email = [var.access_owner_email]
  }
}
