# boki3-quiz デプロイ手順

feedflow (feedflow-go-htmx) が使用中の既存EC2に同居する構成です。AWSリソースは一切作成せず、Cloudflare側のリソース (DNS / Origin CA証明書 / Access) と既存EC2へのアプリ配置だけをterraformで管理します。

## 構成

```text
ブラウザ
  → Cloudflare (DNSプロキシ / エッジHTTPS終端 / Access認証: 本人メールのみ)
    → EC2 (feedflowのterraformが所有: t4g.micro, EIP。SGはアプリポート443/80をCloudflareエッジIPのみに、SSHは運用者のIP/32のみに個別に制限)
      → feedflowのnginx (443をSNIで振り分け。/etc/feedflow/conf.d/boki3.conf をドロップイン)
        → boki3-app コンテナ (共有ネットワーク feedflow_internal 上、ポート非公開)
          → SQLite (/mnt/feedflow-data/boki3 に永続化。feedflowのデータ用EBSを共用)
```

- feedflowリポジトリへの変更はゼロです。nginxが `conf.d/*.conf` を丸ごとincludeし、`/etc/feedflow/conf.d` と `/etc/feedflow/tls` をホストからマウントしているため、ファイルを置くだけで済みます。
- boki3のnginx serverブロックはDocker内蔵DNS (`resolver 127.0.0.11`) + 変数proxy_passで遅延解決するため、boki3が停止していてもfeedflowのnginxの起動・reloadは失敗しません。
- アプリの `.env` (JWTシークレット) は `/home/ec2-user/boki3.env` に永続化し、初回デプロイ時のみ自動生成します。再デプロイでセッションは無効になりません。
- 問題データは起動時のauto-seed (冪等) で投入されるため、手動のシード作業は不要です。

## 前提

- feedflow側のterraformが適用済みで、EC2 / nginx / `feedflow_internal` ネットワーク / `/mnt/feedflow-data` が稼働していること
- ローカルに feedflow リポジトリがあり、`deploy/terraform/feedflow_ssh_key.pem` が存在すること
- **実行元のグローバルIPがfeedflowのセキュリティグループのSSH許可CIDRと一致していること。** feedflowのSGは「feedflow側で最後に `terraform apply` した時点のグローバルIP/32」だけにSSHを許可しています。ネットワークが変わっている場合、boki3のapplyはSSH接続タイムアウトで失敗します。その場合は先にfeedflow側で `terraform apply` を実行してSGの許可CIDRを現在のIPへ更新してください。

## 手順

```bash
cd deploy/terraform
cp secrets.auto.tfvars.example secrets.auto.tfvars
# secrets.auto.tfvars に APIトークン / アカウントID / EIP を記入する
# EIPは feedflowリポジトリで: terraform output -raw elastic_ip

terraform init
terraform plan
terraform apply
```

apply後、`terraform output app_url` のURLへアクセスし、Cloudflare Accessの認証（所有者メールへのOTP等）を通過するとアプリに到達できます。

## 再デプロイ

コード変更後に `terraform apply` を再実行すると、バンドルのチェックサム差分を検知して再転送・再ビルド・再起動します。コード変更なしで強制的に再デプロイしたい場合は `terraform taint null_resource.deploy && terraform apply` を使います。

## 注意

- feedflow側で `terraform apply`（EC2の再作成を伴う変更）を行った場合、boki3の配置物（コンテナ・nginx conf・証明書）は失われるため、boki3側でも `terraform apply` の再実行が必要です（`null_resource.deploy` は `terraform taint null_resource.deploy` で強制再実行できます）。SQLiteのデータは追加EBS上にあるため、同じEBSがアタッチされ続ける限りEC2再作成でも保持されます。失われるのはEBSボリューム自体を作り直した場合だけです。
- feedflowのcomposeを `down` するとき、boki3のコンテナが `feedflow_internal` に接続中だとネットワーク削除に失敗します。先に `cd /home/ec2-user/boki3 && sudo docker compose down` を実行してください。
