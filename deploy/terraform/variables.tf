# 入力変数を定義します。feedflowの既存EC2への同居を前提とします。

variable "project_name" {
  description = "タグや名前の接頭辞に使うプロジェクト名を指定します。"
  type        = string
  default     = "boki3-quiz"
}

variable "origin_public_ip" {
  description = "同居先EC2のElastic IPです。feedflowリポジトリで terraform output elastic_ip を実行して取得します。AレコードとSSH接続先に使います。"
  type        = string
}

variable "ssh_private_key_path" {
  description = "同居先EC2へSSHするための秘密鍵パスです。feedflowのterraformが生成した鍵を再利用します。"
  type        = string
  default     = "../../../feedflow-go-htmx/deploy/terraform/feedflow_ssh_key.pem"
}

# Cloudflare関連の変数です。秘密値はsecrets.auto.tfvarsへ記入し、コードへは書きません。

variable "cloudflare_api_token" {
  description = "DNSとAccessとOrigin CA証明書を操作するCloudflare APIトークンです。DNS編集とゾーン読み取りとSSL and Certificates編集とAccess Apps and Policies編集の権限が要ります。feedflowで発行したトークンを流用できます。"
  type        = string
  sensitive   = true
}

variable "cloudflare_account_id" {
  description = "Cloudflare AccessアプリをひもづけるアカウントIDです。"
  type        = string
}

variable "zone_name" {
  description = "Cloudflareで管理しているゾーン名です。"
  type        = string
  default     = "okamyuji.work"
}

variable "hostname" {
  description = "アプリを公開する完全修飾ホスト名です。AレコードとAccessとOrigin証明書に使います。"
  type        = string
  default     = "boki3.okamyuji.work"
}

variable "access_owner_email" {
  description = "Cloudflare Accessで通過を許可する所有者のメールアドレスです。"
  type        = string
  default     = "owner@example.com"
}
