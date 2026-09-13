# yadori フロントエンド

| ディレクトリ | サイト | ポート |
|---|---|---|
| `admin/` | 宿の運営者向け管理画面 | 5173 |
| `booker/` | 予約者向けサイト | 5174 |

## 起動

先にAPIサーバーを起動しておく。

```bash
cd ../server
DATABASE_URL="postgres://..." go run ./cmd/api
```

そのうえで、それぞれのディレクトリで:

```bash
npm install
npm run dev
```

APIの接続先は `VITE_API_BASE_URL`、認証の接続先は `VITE_OIDC_AUTHORITY` / `VITE_OIDC_CLIENT_ID` で変えられる
（`.env.example` 参照）。サーバー側は `ALLOWED_ORIGINS` でこの2つのポートを許可している。

ログインは OIDC（Authorization Code + PKCE）で、`src/auth.ts` が `oidc-client-ts` を包んでいる。
API 呼び出しには `src/api/client.ts` が自動でアクセストークンを付ける。

## 構成

デザインを差し替えやすいよう、通信と見た目を分けてある。

```
src/
├── auth.ts          ログイン・トークンの取得（oidc-client-ts）
├── api/client.ts    APIとのやり取り。トークンを自動で付ける。デザインを変えても変わらない
├── components/ui.tsx 見た目の部品
└── pages/           画面
```

Claude Design のデザインを取り込む際は、`components/ui.tsx` と `pages/` を
差し替えれば済むようにしている。
