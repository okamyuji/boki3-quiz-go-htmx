# Static Assets

Self-hosted assets served from `/static/*`. CSP は `script-src 'self' 'nonce-...'` を維持できる。

## JS

| File | Version | Origin | SHA-384 (SRI) |
|---|---|---|---|
| `js/htmx.min.js` | 2.0.4 | https://unpkg.com/htmx.org@2.0.4/dist/htmx.min.js | `HGfztofotfshcF7+8n44JQL2oJmowVChPTg48S+jvZoztPfvwD79OC/LTtG6dMp+` |
| `js/alpine.min.js` | 3.14.8 | https://cdn.jsdelivr.net/npm/alpinejs@3.14.8/dist/cdn.min.js | `X9kJyAubVxnP0hcA+AMMs21U445qsnqhnUF8EBlEpP3a42Kh/JwWjlv2ZcvGfphb` |

更新手順:

```bash
curl -sSL -o internal/transport/web/static/js/htmx.min.js \
  https://unpkg.com/htmx.org@2.0.4/dist/htmx.min.js
curl -sSL -o internal/transport/web/static/js/alpine.min.js \
  https://cdn.jsdelivr.net/npm/alpinejs@3.14.8/dist/cdn.min.js

openssl dgst -sha384 -binary internal/transport/web/static/js/htmx.min.js | openssl base64 -A
openssl dgst -sha384 -binary internal/transport/web/static/js/alpine.min.js | openssl base64 -A
```

## CSS

`css/app.css` は手書きの和モダンスタイル (紙×墨×朱)。oklch + rem + 流体タイポ。
固定 max-width は使わず grid と clamp で組む。reduced-motion 対応済み。

## Fonts

Noto Serif JP / Noto Sans JP は OS フォントへのフォールバックを前提とし、バイナリ未同梱。
配布したい場合は `fonts/` に woff2 を置き `app.css` に `@font-face` を宣言する。
