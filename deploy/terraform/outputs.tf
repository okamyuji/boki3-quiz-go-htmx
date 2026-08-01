# applyの結果として確認に使う値と運用手順を出力します。

output "app_url" {
  description = "公開URLです。Cloudflare Accessとアプリログインのあとにクイズへ入れます。"
  value       = "https://${var.hostname}"
}

output "dns_record" {
  description = "作成したCloudflareのAレコードです。プロキシONでオリジンIPを秘匿します。"
  value       = "${cloudflare_record.app.name} A ${var.origin_public_ip} (proxied, feedflow EC2に同居)"
}

output "access_application" {
  description = "本人限定を担うCloudflare Accessアプリの保護ドメインです。"
  value       = cloudflare_zero_trust_access_application.app.domain
}

output "ssh_command" {
  description = "同居先EC2へSSH接続するコマンドの例です。鍵はfeedflowのものを再利用しています。"
  value       = "ssh -i ${var.ssh_private_key_path} ec2-user@${var.origin_public_ip}"
}

output "healthcheck" {
  description = "公開URLのhealthzへアクセスし200を期待するcurlの例です。Access認証後のセッションが必要です。"
  value       = "curl -sS -o /dev/null -w '%%{http_code}\\n' https://${var.hostname}/healthz"
}
