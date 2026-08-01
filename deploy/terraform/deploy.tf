# アプリの配送と起動を行います。イメージはローカルでarm64ビルドして転送し、
# EC2側は docker load と起動だけを行います。
# 同居先EC2はt4g.micro (1GB RAM) をfeedflowと共有しており、EC2上でのGoビルドは
# メモリを食い尽くして同居アプリを巻き込むため行いません (2026-08-01の障害の再発防止)。
# SSH到達にはfeedflowの鍵を再利用します。
#
# feedflow側の前提 (feedflowのterraformが構築済みであること):
#   - Docker / docker compose が導入済み
#   - composeプロジェクト feedflow が /home/ec2-user/feedflow で稼働し、
#     ネットワーク feedflow_internal と nginx (conf.d全体をinclude) を持つ
#   - /etc/feedflow/conf.d と /etc/feedflow/tls が nginx へマウント済み
#   - データ用EBSが /mnt/feedflow-data へマウント済み

locals {
  repo_root  = abspath("${path.module}/../..")
  tmp_dir    = abspath("${path.module}/.terraform-tmp")
  image_path = "${local.tmp_dir}/boki3-image.tar.gz"

  # ビルド入力のチェックサムでイメージ再ビルドと再デプロイのトリガーを作ります。
  # 埋め込み資産 (テンプレート/静的ファイル/マイグレーション/シード) は
  # internal/ と seed/ の下にあるため、この集合がビルド入力のすべてです。
  build_files = sort(concat(
    tolist(fileset(local.repo_root, "cmd/**")),
    tolist(fileset(local.repo_root, "internal/**")),
    tolist(fileset(local.repo_root, "seed/**")),
    ["go.mod", "go.sum", "Dockerfile", "compose.yml"],
  ))
  build_hash = sha256(join("", [for f in local.build_files : filesha256("${local.repo_root}/${f}")]))

  # nginxのreal_ip復元用にCloudflareの全IP範囲からset_real_ip_from行を組み立てます。
  cloudflare_cidrs = concat(
    data.cloudflare_ip_ranges.cloudflare.ipv4_cidr_blocks,
    data.cloudflare_ip_ranges.cloudflare.ipv6_cidr_blocks,
  )
  real_ip_from_block = join("\n", [for c in local.cloudflare_cidrs : "    set_real_ip_from ${c};"])

  nginx_conf = templatefile("${path.module}/templates/boki3.cloudflare.conf.tftpl", {
    hostname           = var.hostname
    real_ip_from_block = local.real_ip_from_block
  })
}

# arm64イメージをローカルでビルドしてtar.gzへ書き出します。ビルド入力が変わると再生成します。
resource "null_resource" "image" {
  triggers = {
    build_hash = local.build_hash
  }

  provisioner "local-exec" {
    working_dir = local.repo_root
    command     = "mkdir -p ${local.tmp_dir} && docker buildx build --platform linux/arm64 --load -t boki3-quiz:dev . && docker save boki3-quiz:dev | gzip > ${local.image_path}"
  }
}

resource "null_resource" "deploy" {
  # ビルド入力や証明書やnginx confや接続先EIPが変わると再実行します。
  triggers = {
    build_hash = local.build_hash
    cert_id    = cloudflare_origin_ca_certificate.origin.id
    hostname   = var.hostname
    nginx_conf = sha256(local.nginx_conf)
    origin_ip  = var.origin_public_ip
  }

  depends_on = [null_resource.image]

  connection {
    type        = "ssh"
    host        = var.origin_public_ip
    user        = "ec2-user"
    private_key = file(var.ssh_private_key_path)
  }

  # ビルド済みイメージとcompose定義を転送します。
  provisioner "file" {
    source      = local.image_path
    destination = "/home/ec2-user/boki3-image.tar.gz"
  }

  provisioner "file" {
    source      = "${local.repo_root}/compose.yml"
    destination = "/home/ec2-user/boki3-compose.yml"
  }

  # nginxのserverブロックを転送します。feedflowのconf.dへドロップインします。
  provisioner "file" {
    content     = local.nginx_conf
    destination = "/home/ec2-user/boki3.cloudflare.conf"
  }

  # Origin CA証明書とその秘密鍵を転送します。Full(strict)でCloudflareが検証する証明書です。
  provisioner "file" {
    content     = cloudflare_origin_ca_certificate.origin.certificate
    destination = "/home/ec2-user/boki3.crt"
  }

  provisioner "file" {
    content     = tls_private_key.origin.private_key_pem
    destination = "/home/ec2-user/boki3.key"
  }

  # 展開とセットアップと起動を行います。feedflow側の資産は変更せず、追加だけを行います。
  provisioner "remote-exec" {
    inline = [
      "set -eux",

      # 転送直後の一時秘密鍵の権限を最初に絞ります (fileプロビジョナは既定umaskで書くため)。
      "chmod 600 /home/ec2-user/boki3.key",

      # 前提確認: feedflowのネットワークとnginxコンテナが稼働中であることを先に検証します。
      "sudo docker network inspect feedflow_internal >/dev/null",
      "cd /home/ec2-user/feedflow && sudo docker compose -f compose.yml -f compose.override.yml ps --services --status running | grep -qx nginx",

      # データディレクトリを用意します。コンテナはdistroless nonroot (uid/gid 65532) で動きます。
      "sudo mkdir -p /mnt/feedflow-data/boki3",
      "sudo chown 65532:65532 /mnt/feedflow-data/boki3",

      # ビルド済みイメージを取り込みます。EC2上ではビルドを行いません。
      "sudo docker load -i /home/ec2-user/boki3-image.tar.gz",
      "rm -f /home/ec2-user/boki3-image.tar.gz",

      # composeプロジェクトを配置します。.envは配置先の外 (boki3.env) に永続化し、再デプロイで消しません。
      "rm -rf /home/ec2-user/boki3 && mkdir -p /home/ec2-user/boki3",
      "mv /home/ec2-user/boki3-compose.yml /home/ec2-user/boki3/compose.yml",

      # JWTシークレットは初回のみ生成します (再生成すると既存セッション/トークンが無効になるため)。
      "test -f /home/ec2-user/boki3.env || printf 'BOKI3_JWT_SECRET=%s\\n' \"$(openssl rand -hex 64)\" > /home/ec2-user/boki3.env",
      "chmod 600 /home/ec2-user/boki3.env",
      "cp /home/ec2-user/boki3.env /home/ec2-user/boki3/.env",
      "chmod 600 /home/ec2-user/boki3/.env",

      # 転送済みイメージで起動します。EC2上でのビルドは行いません (--no-build)。
      "cd /home/ec2-user/boki3 && sudo docker compose --env-file .env up -d --no-build",

      # Origin CA証明書と鍵を配置します。鍵は所有者のみ読めるようにします。
      "sudo cp /home/ec2-user/boki3.crt /etc/feedflow/tls/boki3.crt",
      "sudo cp /home/ec2-user/boki3.key /etc/feedflow/tls/boki3.key",
      "sudo chmod 600 /etc/feedflow/tls/boki3.key",
      "rm -f /home/ec2-user/boki3.crt /home/ec2-user/boki3.key",

      # nginx confをドロップインします。検証失敗時は直前の正常なconfへロールバックし、
      # 無効なファイルを共有conf.dへ残さないことでfeedflowのnginx再起動を壊さないようにします。
      "if test -f /etc/feedflow/conf.d/boki3.conf; then sudo cp /etc/feedflow/conf.d/boki3.conf /home/ec2-user/boki3.conf.bak; fi",
      "sudo cp /home/ec2-user/boki3.cloudflare.conf /etc/feedflow/conf.d/boki3.conf",
      "if ! (cd /home/ec2-user/feedflow && sudo docker compose -f compose.yml -f compose.override.yml exec -T nginx nginx -t); then if [ -f /home/ec2-user/boki3.conf.bak ]; then sudo cp /home/ec2-user/boki3.conf.bak /etc/feedflow/conf.d/boki3.conf; else sudo rm -f /etc/feedflow/conf.d/boki3.conf; fi; echo 'nginx config validation failed; boki3.conf rolled back' >&2; exit 1; fi",
      "rm -f /home/ec2-user/boki3.conf.bak",
      "cd /home/ec2-user/feedflow && sudo docker compose -f compose.yml -f compose.override.yml exec -T nginx nginx -s reload",

      # 起動状態とアプリの実応答を確認します。appコンテナのIPへホストから直接healthzを叩きます。
      "cd /home/ec2-user/boki3 && sudo docker compose ps",
      "cd /home/ec2-user/boki3 && APP_CID=$(sudo docker compose ps -q boki3-app | head -n1) && APP_IP=$(sudo docker inspect -f '{{range .NetworkSettings.Networks}}{{.IPAddress}}{{end}}' \"$APP_CID\") && curl -fsS -m 10 --retry 5 --retry-delay 2 --retry-connrefused -o /dev/null \"http://$APP_IP:8080/healthz\"",
    ]
  }
}
