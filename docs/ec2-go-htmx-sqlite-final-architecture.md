# EC2単体構成による Go + HTMX + SQLite Webアプリ運用案

作成日: 2026-05-24  
対象リージョン: 東京リージョン `ap-northeast-1`  
対象アプリ: 1ユーザーのみが利用する Go + HTMX + SQLite3 Webアプリ  
基本方針: RDS / ECS / ALB / EFS を使わず、EC2単体で低コストに運用する

---

## 1. 結論

1ユーザーだけが利用する軽量な Go + HTMX + SQLite3 アプリであれば、最終案は以下が現実的です。

```text
Internet
  ↓ 80/443
EC2: t4g.nano または t4g.micro
  ├── nginx: TLS終端 / rate limit / reverse proxy
  ├── Go app: 127.0.0.1:8080 のみで待ち受け
  ├── SQLite: /var/lib/myapp/app.sqlite
  ├── EBS gp3: OS + アプリ + SQLite DB
  └── systemd timer: 日次バックアップをS3へ退避
```

推奨は、安定寄りなら **t4g.micro + EBS gp3 20GB**、最小コスト寄りなら **t4g.nano + EBS gp3 8〜16GB** です。

本構成の狙いは、DBサーバーやコンテナオーケストレーションを避け、SQLiteの長所である「単一ファイルDB」を活かすことです。可用性や水平スケールは捨て、**障害時は直近バックアップへ戻す** という運用に割り切ります。

---

## 2. 設計方針

### 2.1 採用するもの

| 項目 | 採用案 |
|---|---|
| コンピュート | EC2 t4g.nano または t4g.micro |
| OS | Amazon Linux 2023 |
| Webフロント | nginx |
| アプリ | Go単体バイナリ |
| UI | HTMX |
| DB | SQLite3 |
| DB配置 | EBS gp3上のローカルファイル |
| TLS | nginx + certbot / acme.sh など |
| 常駐管理 | systemd |
| バックアップ | SQLite `.backup` → gzip → S3 |
| 復旧方針 | 直近バックアップ時点へ戻す |

### 2.2 採用しないもの

| 不採用 | 理由 |
|---|---|
| RDS | 1ユーザー用途では固定費・運用が過剰 |
| ECS/Fargate | SQLite永続化と固定URL/TLS設計が相対的に複雑 |
| ALB | 1ユーザー用途では固定費が大きい |
| EFS | SQLite用途ではネットワークFSの注意点が残る |
| S3 Files上のライブSQLite | 可能性はあるが、単純性と検証コストの観点でEC2 + EBSの方が素直 |
| マルチAZ | 障害時はバックアップ復旧に割り切るため不要 |

---

## 3. 月額費用の目安

> 注意: AWS料金は変更される可能性があります。ここでは2026-05-24時点で公式料金ページおよびAWS料金体系に基づく概算として整理しています。最終確認は AWS Pricing Calculator または公式料金ページで行ってください。

### 3.1 t4g.nano 最小構成

| 費目 | 前提 | 月額概算 |
|---|---:|---:|
| EC2 t4g.nano | 730h/月 | 約 $3.94 |
| EBS gp3 8GB | $0.096/GB-month想定 | 約 $0.77 |
| Public IPv4 | $0.005/IP-hour × 730h | 約 $3.65 |
| Data Transfer IN | 無料 | $0.00 |
| Data Transfer OUT | 月100GBまで無料枠内想定 | $0.00 |
| S3バックアップ | 小容量・短期保持想定 | 数セント〜 |
| 合計 |  | **約 $8.4/月 + S3少額** |

### 3.2 t4g.micro 推奨構成

| 費目 | 前提 | 月額概算 |
|---|---:|---:|
| EC2 t4g.micro | 730h/月 | 約 $7.88 |
| EBS gp3 20GB | $0.096/GB-month想定 | 約 $1.92 |
| Public IPv4 | $0.005/IP-hour × 730h | 約 $3.65 |
| Data Transfer IN | 無料 | $0.00 |
| Data Transfer OUT | 月100GBまで無料枠内想定 | $0.00 |
| S3バックアップ | 小容量・短期保持想定 | 数セント〜数十セント程度 |
| 合計 |  | **約 $13.45/月 + S3少額** |

### 3.3 補足

Public IPv4は、2024-02-01以降、利用中か未使用かに関わらずパブリックIPv4アドレスに時間課金が発生します。EC2本体が小さい場合、Public IPv4料金は無視できない固定費になります。

Data Transfer IN は無料です。Data Transfer OUT は AWSサービス・リージョン横断で月100GBまで無料枠があります。1ユーザー向けHTMXアプリで画像・動画を大量配信しないなら、多くの場合この無料枠内に収まる想定です。

---

## 4. アーキテクチャ詳細

### 4.1 ネットワーク構成

```text
User Browser
  ↓ HTTPS
Security Group: 443 open
  ↓
nginx on EC2
  ↓ proxy_pass
Go app on 127.0.0.1:8080
  ↓
SQLite on /var/lib/myapp/app.sqlite
```

Security Groupは最小にします。

| Port | Source | 用途 |
|---:|---|---|
| 22 | 自分の固定IPのみ | SSH |
| 80 | 0.0.0.0/0, ::/0 | HTTP→HTTPSリダイレクト、証明書更新 |
| 443 | 0.0.0.0/0, ::/0 | HTTPS |
| 8080 | 開けない | Goアプリはlocalhostのみ |

### 4.2 ディレクトリ構成

```text
/opt/myapp/
  ├── myapp                 # Goバイナリ
  ├── static/               # CSS/JS/画像など
  └── templates/            # 必要ならテンプレート

/var/lib/myapp/
  └── app.sqlite            # SQLite DB

/var/log/myapp/
  └── app.log               # アプリログ

/var/backups/myapp/
  └── app-YYYY-MM-DD-HHMMSS.sqlite.gz
```

---

## 5. nginx設定案

以下は基本形です。実ドメインに合わせて `example.com` を置き換えてください。

```nginx
limit_req_zone $binary_remote_addr zone=per_ip:10m rate=5r/s;
limit_conn_zone $binary_remote_addr zone=addr:10m;

server {
    listen 80;
    server_name example.com;

    location /.well-known/acme-challenge/ {
        root /var/www/letsencrypt;
    }

    location / {
        return 301 https://$host$request_uri;
    }
}

server {
    listen 443 ssl http2;
    server_name example.com;

    ssl_certificate     /etc/letsencrypt/live/example.com/fullchain.pem;
    ssl_certificate_key /etc/letsencrypt/live/example.com/privkey.pem;

    client_max_body_size 10m;

    limit_req zone=per_ip burst=20 nodelay;
    limit_conn addr 20;

    add_header X-Frame-Options "DENY" always;
    add_header X-Content-Type-Options "nosniff" always;
    add_header Referrer-Policy "strict-origin-when-cross-origin" always;

    location /static/ {
        alias /opt/myapp/static/;
        access_log off;
        expires 7d;
    }

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_http_version 1.1;

        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;

        proxy_read_timeout 60s;
        proxy_send_timeout 60s;
    }
}
```

### 5.1 nginxで担う責務

| 責務 | 内容 |
|---|---|
| TLS終端 | Goアプリに証明書管理を持ち込まない |
| Rate limit | 簡易的な乱用・ブルートフォース対策 |
| リクエストサイズ制限 | 不要な大容量POSTを防ぐ |
| 静的ファイル配信 | Goアプリの負荷を下げる |
| セキュリティヘッダ | 最低限のブラウザ防御 |
| アプリ隠蔽 | Goアプリを外部公開しない |

---

## 6. Goアプリの起動設定

Goアプリは必ずlocalhostで待ち受けます。

```bash
/opt/myapp/myapp -addr 127.0.0.1:8080
```

systemd unit例:

```ini
[Unit]
Description=My Go HTMX App
After=network.target

[Service]
User=myapp
Group=myapp
WorkingDirectory=/opt/myapp
ExecStart=/opt/myapp/myapp -addr 127.0.0.1:8080
Restart=always
RestartSec=3

Environment=APP_ENV=production
Environment=APP_DB_PATH=/var/lib/myapp/app.sqlite

NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=full
ProtectHome=true
ReadWritePaths=/var/lib/myapp /var/log/myapp

[Install]
WantedBy=multi-user.target
```

ユーザー作成例:

```bash
sudo useradd --system --home /nonexistent --shell /sbin/nologin myapp
sudo mkdir -p /opt/myapp /var/lib/myapp /var/log/myapp /var/backups/myapp
sudo chown -R myapp:myapp /var/lib/myapp /var/log/myapp
```

---

## 7. SQLite設定方針

### 7.1 基本方針

SQLiteはEBS上のローカルファイルとして配置します。

```text
/var/lib/myapp/app.sqlite
```

1ユーザー用途であれば、同時書き込みは少ない想定です。アプリ側で書き込みを必要以上に並列化しない設計にします。

### 7.2 推奨PRAGMA例

```sql
PRAGMA journal_mode=WAL;
PRAGMA synchronous=NORMAL;
PRAGMA busy_timeout=5000;
PRAGMA foreign_keys=ON;
```

EC2 + EBS上のローカルSQLiteであれば、WALは一般的に使いやすい選択肢です。読み取りと書き込みの並行性が少し改善します。ただし、バックアップ時にはSQLite公式の `.backup` APIまたは `VACUUM INTO` を利用し、DBファイルだけを雑に `cp` しないようにします。

### 7.3 注意点

| 注意点 | 内容 |
|---|---|
| 複数プロセス書き込み | できるがロック待ちに注意 |
| DBファイル直接コピー | 稼働中は避ける |
| WALファイル | `app.sqlite-wal` / `app.sqlite-shm` が作成される |
| バックアップ | `.backup` または `VACUUM INTO` を使う |
| 復旧点 | 日次バックアップ時点まで |

---

## 8. バックアップ設計

### 8.1 方針

この構成では、高可用性よりも低コスト・単純性を優先します。

```text
障害発生
  ↓
EC2またはEBSを再作成
  ↓
S3から最新バックアップを取得
  ↓
/var/lib/myapp/app.sqlite に戻す
  ↓
systemdでGoアプリ起動
```

バックアップ後の更新分は復旧しません。これは設計上の割り切りです。

### 8.2 バックアップスクリプト例

```bash
#!/usr/bin/env bash
set -euo pipefail

APP_DB="/var/lib/myapp/app.sqlite"
BACKUP_DIR="/var/backups/myapp"
BUCKET="s3://your-backup-bucket/myapp/sqlite"
DATE="$(date +%F-%H%M%S)"
OUT="${BACKUP_DIR}/app-${DATE}.sqlite"

mkdir -p "$BACKUP_DIR"

sqlite3 "$APP_DB" ".backup '${OUT}'"
gzip "$OUT"

aws s3 cp "${OUT}.gz" "${BUCKET}/app-${DATE}.sqlite.gz"

# ローカルは7日分だけ保持
find "$BACKUP_DIR" -type f -name 'app-*.sqlite.gz' -mtime +7 -delete
```

### 8.3 systemd timer例

`/etc/systemd/system/myapp-backup.service`

```ini
[Unit]
Description=Backup MyApp SQLite database

[Service]
Type=oneshot
ExecStart=/usr/local/bin/myapp-backup.sh
```

`/etc/systemd/system/myapp-backup.timer`

```ini
[Unit]
Description=Daily MyApp SQLite backup

[Timer]
OnCalendar=*-*-* 03:00:00
Persistent=true

[Install]
WantedBy=timers.target
```

有効化:

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now myapp-backup.timer
systemctl list-timers | grep myapp
```

### 8.4 S3バックアップ保持

S3 Lifecycleで保持期間を決めます。

| 方針 | 例 |
|---|---|
| 最小 | 7世代 |
| 安全寄り | 14〜30世代 |
| 長期保管 | 月初分だけ1年保持 |

1ユーザー用途では、まずは **日次7〜14世代** で十分なことが多いです。

---

## 9. 復旧手順

### 9.1 EC2だけ壊れた場合

1. 新しいEC2を作成する
2. セキュリティグループを付与する
3. nginx / sqlite / awscli などをインストールする
4. Goバイナリを配置する
5. S3から最新DBバックアップを取得する
6. `/var/lib/myapp/app.sqlite` に展開する
7. systemdでアプリ起動
8. nginxを起動
9. DNSまたはElastic IPを新インスタンスに向ける

### 9.2 DB復元コマンド例

```bash
aws s3 cp s3://your-backup-bucket/myapp/sqlite/app-YYYY-MM-DD-HHMMSS.sqlite.gz /tmp/
gunzip /tmp/app-YYYY-MM-DD-HHMMSS.sqlite.gz
sudo install -o myapp -g myapp -m 0600 /tmp/app-YYYY-MM-DD-HHMMSS.sqlite /var/lib/myapp/app.sqlite
sudo systemctl restart myapp
```

---

## 10. セキュリティ運用

### 10.1 最低限やること

| 項目 | 推奨 |
|---|---|
| SSH | 公開鍵のみ |
| SSH接続元 | 自分の固定IPに制限 |
| rootログイン | 無効化 |
| パスワードログイン | 無効化 |
| nginx | 80/443のみ公開 |
| Goアプリ | 127.0.0.1のみ待ち受け |
| OS更新 | 定期実施 |
| ログ | logrotate |
| バックアップ | 自動化 + 復元テスト |
| IAM Role | S3バックアップ先への最小権限 |

### 10.2 IAMポリシーの考え方

EC2にはアクセスキーを置かず、IAM Roleを付与します。S3バックアップ用バケットに対して、必要最小限の操作だけ許可します。

必要な操作例:

```json
{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Effect": "Allow",
      "Action": [
        "s3:PutObject",
        "s3:GetObject",
        "s3:ListBucket"
      ],
      "Resource": [
        "arn:aws:s3:::your-backup-bucket",
        "arn:aws:s3:::your-backup-bucket/myapp/*"
      ]
    }
  ]
}
```

---

## 11. 監視

1ユーザー用途では、CloudWatchを最小限使えば十分です。

| 対象 | 方法 |
|---|---|
| EC2死活 | CloudWatch Status Check |
| ディスク使用量 | node exporter / cron通知 / CloudWatch Agent |
| nginx error log | logrotate + 必要に応じてCloudWatch Logs |
| アプリ異常終了 | systemd `Restart=always` |
| バックアップ失敗 | systemd timerの失敗通知、またはログ監視 |
| 証明書期限 | certbot timer確認、または外部監視 |

コストを抑えるなら、最初は以下で十分です。

```bash
systemctl status myapp
systemctl status nginx
systemctl list-timers
journalctl -u myapp -n 100
journalctl -u myapp-backup.service -n 100
```

---

## 12. デプロイ手順

### 12.1 ローカルでビルド

ARM向けEC2 t4gを使う場合:

```bash
GOOS=linux GOARCH=arm64 go build -o myapp ./cmd/myapp
```

### 12.2 EC2へ配置

```bash
scp myapp ec2-user@example.com:/tmp/myapp
ssh ec2-user@example.com
sudo install -o root -g root -m 0755 /tmp/myapp /opt/myapp/myapp
sudo systemctl restart myapp
```

### 12.3 ロールバック

```bash
sudo cp /opt/myapp/myapp /opt/myapp/myapp.prev
sudo install -o root -g root -m 0755 /tmp/myapp /opt/myapp/myapp
sudo systemctl restart myapp
```

失敗時:

```bash
sudo mv /opt/myapp/myapp.prev /opt/myapp/myapp
sudo systemctl restart myapp
```

---

## 13. リスクと割り切り

| リスク | 対応方針 |
|---|---|
| EC2障害 | 新規EC2 + バックアップ復元 |
| EBS障害 | S3バックアップから復元 |
| バックアップ後のデータ消失 | 許容する |
| AZ障害 | 復旧まで停止を許容する |
| DDoS | 本格対策はしない。必要ならCloudFront/WAFへ拡張 |
| 管理ミス | 復元手順をドキュメント化し、定期テスト |
| Public IPv4費用 | 必要経費として受け入れる。IPv6のみ運用は条件付き |

---

## 14. 将来の拡張ポイント

### 14.1 少し利用者が増えた場合

```text
EC2 t4g.micro → t4g.small / t4g.medium
EBS gp3容量拡張
nginx rate limit調整
SQLite busy_timeout調整
```

### 14.2 DBがボトルネックになった場合

```text
SQLite → PostgreSQL / MySQL
EC2内DB → RDS
```

### 14.3 可用性が必要になった場合

```text
EC2単体 → ALB + 複数EC2
SQLite → RDS / Aurora
静的ファイル → S3 + CloudFront
```

### 14.4 セキュリティを上げたい場合

```text
CloudFront + WAF
SSM Session ManagerでSSH廃止
CloudWatch Logs集約
AWS Backup / AMI定期作成
```

---

## 15. 最終推奨構成

### バランス重視

```text
EC2: t4g.micro
OS: Amazon Linux 2023
Disk: EBS gp3 20GB
Web: nginx
App: Go binary + systemd
DB: SQLite on /var/lib/myapp/app.sqlite
Backup: daily SQLite .backup → gzip → S3, 14世代
TLS: Let's Encrypt
```

### コスト最小重視

```text
EC2: t4g.nano
OS: Amazon Linux 2023
Disk: EBS gp3 8〜16GB
Web: nginx
App: Go binary + systemd
DB: SQLite
Backup: daily SQLite .backup → S3, 7世代
```

---

## 16. 事実として確度が高い部分

- EC2単体構成にすることで、RDS/ECS/ALB/EFSの固定費を避けられる。
- 1ユーザー向けのGo + HTMX + SQLiteアプリでは、EC2 + EBS + SQLiteは構成が単純で相性がよい。
- nginxを前段に置くことで、TLS終端、rate limit、reverse proxy、リクエストサイズ制限をまとめて扱える。
- Goアプリは `127.0.0.1` のみで待ち受け、外部にはnginxだけを公開する構成がよい。
- SQLiteの稼働中バックアップには、単純なファイルコピーではなく `.backup` などのSQLite対応手段を使うべき。
- Public IPv4は時間課金されるため、小型EC2では無視できない固定費になる。
- Data Transfer INは無料、Data Transfer OUTは月100GB無料枠がある。

---

## 17. 誤解されやすい部分

- 「EBSの耐性を下げると大幅に安くなる」というより、RDS/ECS/ALB/EFSを使わないことで安くなる。
- nginxを置いても、アプリ側の認証、認可、CSRF対策、入力検証は必要。
- rate limitは簡易防御であり、大規模DDoS対策ではない。
- 日次バックアップ運用では、バックアップ後の更新は失われる。
- t4g.nanoは非常に安いが、メモリ0.5GiBのためOS更新、圧縮、ログ、nginx同居で窮屈になる可能性がある。

---

## 18. 条件次第で変わる部分

- 月100GBを超えるアウトバウンド通信がある場合、データ転送料金が効いてくる。
- 画像や動画を配信するなら、S3 + CloudFrontの併用を検討する。
- 複数ユーザーや高頻度書き込みが発生するなら、SQLiteからRDS/PostgreSQLへ移行する方がよい。
- RTOを短くしたいなら、AMI定期作成やEBS Snapshotを追加する。
- SSHを公開したくない場合は、SSM Session Managerを使う。
- IPv6のみで運用できるならPublic IPv4費用を削減できるが、利用者側のネットワーク対応が前提になる。

---

## 19. 参考公式情報

- Amazon EC2 On-Demand Pricing: https://aws.amazon.com/ec2/pricing/on-demand/
- Amazon EC2 T4g Instances: https://aws.amazon.com/ec2/instance-types/t4/
- Amazon EBS Pricing: https://aws.amazon.com/ebs/pricing/
- AWS Public IPv4 Address Charge: https://aws.amazon.com/blogs/aws/new-aws-public-ipv4-address-charge-public-ip-insights/
- AWS Data Transfer / 100GB Free DTO: https://aws.amazon.com/ec2/pricing/on-demand/
- SQLite Backup API: https://www.sqlite.org/backup.html
- SQLite WAL: https://www.sqlite.org/wal.html
- nginx rate limiting: https://nginx.org/en/docs/http/ngx_http_limit_req_module.html
- nginx reverse proxy: https://docs.nginx.com/nginx/admin-guide/web-server/reverse-proxy/

---

## 20. 最終判断

この用途では、**EC2単体 + nginx + Go/HTMX + SQLite + EBS gp3 + 日次S3バックアップ** が最も合理的です。

本構成は、以下を明確に割り切ることで成立します。

```text
高可用性より低コスト
水平スケールより単純性
リアルタイム復旧より日次バックアップ復旧
DBサーバー運用よりSQLite単一ファイル運用
```

実装開始時の第一候補は **t4g.micro + EBS gp3 20GB**、コスト最小を優先する場合は **t4g.nano + EBS gp3 8〜16GB** です。
