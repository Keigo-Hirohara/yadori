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

APIの接続先は `VITE_API_BASE_URL` で変えられる（既定は `http://localhost:8080/api/v1`）。
サーバー側は `ALLOWED_ORIGINS` でこの2つのポートを許可している。

## 構成

デザインを差し替えやすいよう、通信と見た目を分けてある。

```
src/
├── api/client.ts    APIとのやり取り。デザインを変えても変わらない
├── components/ui.tsx 見た目の部品
└── pages/           画面
```

Claude Design のデザインを取り込む際は、`components/ui.tsx` と `pages/` を
差し替えれば済むようにしている。
