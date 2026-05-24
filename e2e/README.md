# E2E (Playwright)

第三者が手元で動作検証できる Playwright E2E スイートです。

## 前提

- Go 1.25 以上
- Node.js 20 以上
- macOS / Linux

## 1 回限りのセットアップ

```bash
cd e2e/playwright
npm install
npm run install:browsers   # chromium だけ取得
```

## 実行

```bash
cd e2e/playwright
npm test                   # ヘッドレス
npm run test:headed        # 画面付き
```

`playwright.config.ts` の `webServer` が

1. プロジェクトルートで `e2e/boot-server.sh` を起動
2. 一時 DB (`/tmp/boki3-e2e-<unix>.db`) に 470 問を投入
3. `http://127.0.0.1:18080` で Go サーバを起動

までを自動で行います。`/healthz` が 200 を返したらテスト開始。

## レポート

実行後 `e2e/playwright/playwright-report/` に HTML レポートと JSON 結果が生成されます。

```bash
npx playwright show-report
```

失敗時は同ディレクトリにスクリーンショット / 動画 / トレース zip が残ります。

## カバーするフロー

| ファイル | 内容 |
|---|---|
| `01-smoke.spec.ts` | home / healthz / version / security headers / 未認証リダイレクト |
| `02-register-login.spec.ts` | 登録 → ログアウト → ログイン、弱パスワード拒否、ゴーストユーザ拒否 |
| `03-quiz-flow.spec.ts` | 仕訳問題の正解 / 不正解、採点結果ページ表示 |
| `04-history-and-progress.spec.ts` | 履歴一覧、個別削除、一括削除、進捗 SVG レンダリング |
| `05-settings-password.spec.ts` | パスワード変更 (他セッション破棄、旧 PW 拒否、新 PW で再ログイン) |
| `06-api-jwt.spec.ts` | `/api/v1/*` JWT bearer フル CRUD、401 系 |

## CI 連携

GitHub Actions では `e2e/playwright/playwright.config.ts` を以下のように差し替え可能です。

```yaml
- name: Playwright
  run: |
    cd e2e/playwright
    npm ci
    npx playwright install --with-deps chromium
    npm test
```
