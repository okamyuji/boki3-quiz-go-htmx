# boki3-quiz-go-htmx

日商簿記検定 3 級学習 Web アプリです。Go 1.25 と SQLite、HTMX、Alpine.js のみで実装し、Web UI は Cookie セッション、JSON API は JWT Bearer の二重認証を提供します。

> 出題範囲は商工会議所の公式区分表 (2022 年度版・2026 年度試験適用) に準拠します。
> 参考: https://www.kentei.ne.jp/bookkeeping/exam-list

## 必要環境

- Go 1.25
- (任意) Node 22 (フロントエンド unit test 実行に使用)
- (任意) pre-commit, staticcheck, golangci-lint v2.12.2, govulncheck v1.3.0, gitleaks 8.30.1

## セットアップ

```bash
cp .env.example .env
# .env 内の BOKI3_JWT_SECRET を 32 バイト以上のランダム値に書き換える
make build
make run
```

`/healthz` `/readyz` `/version` がデフォルトで `:8080` で応答します。

## 品質ゲート

pre-commit と CI は同じ `scripts/quality-gate.sh` を呼びます。ローカルでも以下で実行できます。

```bash
make quality-gate
```

gofmt / go vet / staticcheck / golangci-lint / govulncheck / SQL Sprintf guard / go test / リリースビルド / gitleaks / (任意で E2E) を順に実行します。

## 開発ドキュメント

- 設計書: `docs/superpowers/specs/2026-05-24-boki3-quiz-design.md`
- 実装ロードマップ: `docs/superpowers/plans/2026-05-24-boki3-quiz-roadmap.md`

## ライセンス

MIT (`LICENSE` 参照)
