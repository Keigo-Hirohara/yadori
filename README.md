# yadori

宿泊施設の空室検索・予約を行うWebアプリケーションです。
ドメイン駆動設計に基づいて開発されたバックエンドが主な技術要素です。

**技術的な主題は、ダブルブッキングを起こさない在庫管理の設計と実装です。**
並行制御、失敗経路の洗い出し、それらを検証するテストが本プロジェクトの中心にあります。

---

## 技術スタック

| 領域 | 選定 |
|---|---|
| 言語 | Go |
| DB | PostgreSQL 16 |
| DB層 | sqlc + pgx/v5 |
| HTTP | 標準 `net/http` |
| マイグレーション | golang-migrate |
| テスト | 標準 `testing` + testify + testcontainers |

ORM は使用していません。ドメインモデルを採用しており、
構造体タグでテーブルと対応させる方式は「集約が永続化を知らない」という設計方針と衝突するためです。

---

## 設計の概要

### 業務領域の分類

事業活動を業務領域に分け、中核・一般・補完に分類した上で、
分類に応じて実装方式を選択しています。

| 業務領域 | 分類 | 実装方式 |
|---|---|---|
| 在庫管理 | 一般（※） | ドメインモデル |
| 予約管理 | 中核 | ドメインモデル |
| 施設情報の管理 | 補完 | アクティブレコード |
| 会員管理 | 補完 | アクティブレコード |
| 決済・認証 | 一般 | 外部サービス＋腐敗防止層 |

※ 事業としては一般に分類されますが、本プロジェクトの目的（並行制御の設計と実装の実践）により、
意図的に中核として自作しています。この2つの判断は別の理由に基づくものです。

### 区切られた文脈

言葉の意味が変わる場所で境界を引き、4つの文脈に分割しています。

```
施設情報の管理   → Accommodation, RoomType
販売枠の管理     → Inventory
予約管理         → Booking
会員管理         → Booker
```

同じ「部屋タイプ」でも、施設情報では商品の定義、販売枠では日付と結びついた枠を指します。

### 在庫集約の境界

**部屋タイプ × 日付** を1インスタンスとしています。

- 宿単位にすると、12/24を予約する人と3/10を予約する人が同じロックを奪い合う
- 全期間を1集約にすると、予約が日付をまたぐため芋づる式に肥大化する

集約の境界がロックの粒度でもある、という理解に基づく判断です。

### ダブルブッキング防止

多層で防御しています。

| 層 | 手段 |
|---|---|
| ドメインモデル | 在庫集約が確保数と販売可能数を検証 |
| トランザクション | `SELECT ... FOR UPDATE` による行ロック |
| DB制約 | `UNIQUE (room_type_id, date, slot_no)` |

各確保に 1〜販売可能数 の枠番号を割り当て、一意制約で二重取得を防ぎます。
カウンタを持たないため、集約の状態とDBがズレる余地がありません。

アプリケーション層の検証だけでは同時実行を防げないため、最終防衛線をDBに置いています。

### 結果整合性

在庫と予約は別の集約であり、1トランザクション1集約の原則を守っています。
複数集約にまたがる予約処理は、時限付き確保とプロセスマネージャーによる補償で整合性を担保します。

```
在庫を確保（日数分）→ 予約を作成 → 決済 → 確定
      ↓ 失敗時
   確保を解放（補償）。またはが期限切れで自動回収
```

決済は外部サービスのためトランザクションが届きません。
「課金は成立したが予約が確定していない」状態が原理的に避けられないため、
Webhook との二重経路と定期照合による復旧手段を用意しています。

---

## セットアップ

### 必要なもの

- Go 1.23+
- Node.js 20+
- Docker（PostgreSQL・Keycloak・テスト用コンテナに使う）

### 起動

```bash
# 依存をそろえる（最初に1度）
make setup

# PostgreSQL と Keycloak（認証）を起動してマイグレーションを適用
make up
make migrate

# 動作確認用のデモデータを入れる（宿3件・部屋タイプ6件・60日分の在庫）
make seed

# API・ワーカー・管理画面・予約者向けサイトをまとめて起動
make dev
```

| | URL |
|---|---|
| API | http://localhost:8080 |
| 管理画面（宿の運営者向け） | http://localhost:5173 |
| 予約者向けサイト | http://localhost:5174 |
| Keycloak 管理コンソール | http://localhost:8180 （admin / admin） |

### テスト用アカウント

Keycloak の realm 定義（`auth/realms/`）に含まれています。

| サイト | realm | ユーザー | パスワード |
|---|---|---|---|
| 管理画面 | `yadori-operator` | `operator@example.com` | `password` |
| 予約者向けサイト | `yadori-booker` | `booker@example.com` | `password` |

予約者向けサイトは Keycloak 側でセルフ登録（サインアップ）を許可しています。
運営者は登録を許可していないので、管理コンソールから追加します。
`make seed` のデモデータは `operator@example.com` の宿として登録されます。

`Ctrl-C` で4つとも止まります。個別に動かす場合は `make api` / `make worker` / `make admin` / `make booker`。

接続先は `.env`（`.env.example` をコピー）で変えられます。

### テスト

```bash
# 全テスト（DBを含む。Docker が要る）
make test

# DB不要な高速テストのみ
make test-short
```

並行制御の検証のため `-race` を常に有効にしています。

### コード生成

```bash
make sqlc
```

### 自宅サーバーへのデプロイと公開

全部（PostgreSQL・Keycloak・API・ワーカー・フロント2つ）を Docker Compose で
自宅サーバー1台に載せます。構成は `deploy/compose.yml`、イメージは `server/Dockerfile` と `web/Dockerfile` です。

**外部からの通信は Cloudflare Tunnel 経由です。** サーバー側の `cloudflared` が Cloudflare へ
外向きに接続し、その中を通ってリクエストが届きます。ルーターのポート開放も、
サーバーへの直接の到達性も要りません。TLS は Cloudflare が終端します。
`cloudflared` はサーバー共通の入口なので compose には含めず、各コンテナは `127.0.0.1` のポートに出します。

| ホスト名 | 中身 | トンネルの向き先 |
|---|---|---|
| `yadori.hirohara-keigo.net` | 予約者向けサイト | `http://localhost:5174` |
| `admin-yadori.hirohara-keigo.net` | 管理画面 | `http://localhost:5173` |
| `api-yadori.hirohara-keigo.net` | API | `http://localhost:8080` |
| `auth-yadori.hirohara-keigo.net` | Keycloak | `http://localhost:8180` |

**main に push すると自動でデプロイされます。** CI（テストとビルド）が通ったあと、
サーバー上の GitHub Actions セルフホストランナーが `docker compose up -d --build` を実行します
（`.github/workflows/ci.yml` の `deploy` ジョブ）。ランナーも外向き接続だけです。

#### 初期設定（1回だけ）

**1. ドメインを Cloudflare に移す（DNS だけ。レジストラは Xserver のまま）**

1. Cloudflare にサインアップ → Add a domain → `hirohara-keigo.net` → Free プラン
2. 表示される2つのネームサーバー（`xxx.ns.cloudflare.com`）を控える
3. Xserver ドメインの管理パネル → 対象ドメイン → ネームサーバー設定 →
   「その他のサービスで利用する」→ 上の2つを入力
4. Cloudflare 側が「Active」になるまで待つ（数分〜数時間）

**2. Tunnel を作る**

1. Cloudflare ダッシュボード → Zero Trust → Networks → Tunnels → Create a tunnel → Cloudflared
2. 名前はサーバー名など（トンネルはサーバー全体で1つ）。次の画面に出る
   `cloudflared service install <トークン>` のトークン部分を控える（手順3のスクリプトに渡す）
3. Public Hostname タブで4つ追加する

   | Subdomain | Domain | Service |
   |---|---|---|
   | `yadori` | `hirohara-keigo.net` | `http://localhost:5174` |
   | `admin-yadori` | `hirohara-keigo.net` | `http://localhost:5173` |
   | `api-yadori` | `hirohara-keigo.net` | `http://localhost:8080` |
   | `auth-yadori` | `hirohara-keigo.net` | `http://localhost:8180` |

   DNS レコードは Cloudflare が自動で作ります。同じサーバーに別のアプリを載せるときは、
   ここに行を足すだけです。

   同じトークンで `cloudflared` を2箇所（ホストとコンテナなど）で動かさないこと。
   Cloudflare がリクエストを両方に振り分けるため、片方が向き先に届かないと
   アクセス元によって成功と失敗が分かれます。

**3. サーバーの初期設定**

GitHub で Settings → Actions → Runners → New self-hosted runner → Linux と進み、
表示される `--token XXXX` の値を控える（1時間で失効）。

```bash
ssh home-server
git clone https://github.com/Keigo-Hirohara/yadori.git && cd yadori
./deploy/setup-server.sh <ランナー登録トークン> <トンネルのトークン>
nano ~/.config/yadori/.env
```

スクリプトが Docker と cloudflared のインストール・ランナーの登録・設定ファイルの雛形作成を行います。
`~/.config/yadori/.env` は Git の外にあり、サーバーにだけ置かれます。

```
BOOKER_ORIGIN=https://yadori.hirohara-keigo.net
ADMIN_ORIGIN=https://admin-yadori.hirohara-keigo.net
API_ORIGIN=https://api-yadori.hirohara-keigo.net
AUTH_ORIGIN=https://auth-yadori.hirohara-keigo.net
DB_PASSWORD=（長いランダム文字列）
KC_ADMIN_PASSWORD=（長いランダム文字列）
```

**4. 起動とデモデータ**

main に push するか、手元で `make deploy`。起動後に `make deploy-seed` でデモデータを入れます。

#### 日常の操作

| コマンド | 内容 |
|---|---|
| `git push origin main` | CI → 自動デプロイ |
| `make deploy` | 手動デプロイ（作業ツリーを rsync して起動） |
| `make deploy-logs` / `deploy-ps` / `deploy-down` | ログ・状態・停止 |
| `make deploy-reset` | **DB を含めて全部消して**作り直す。ホスト名を変えたときや初期化したいとき |

接続先は `~/.ssh/config` のホスト名で、`.env` の `DEPLOY_HOST` で指定します（既定は `home-server`）。

#### 本番で変えていること

`deploy/compose.yml` は開発用の `compose.yml` と次の点が違います。

- Keycloak は `start`（本番モード）で、データを PostgreSQL の `keycloak` データベースに置く。
  `KC_HOSTNAME` を公開 URL に固定し、Cloudflare からの `X-Forwarded-*` を信頼する
- realm 定義は `realm-render` が取り込み前に書き換える。リダイレクト先を公開 URL にし、
  開発用の password grant（`directAccessGrantsEnabled`）を無効にする
- ホストに公開するポートは `127.0.0.1` に限定する（トンネルと CD の疎通確認だけが使う）
- Keycloak 用のデータベースは `keycloak-db` が毎回「無ければ作る」。realm の取り込みは realm が無いときだけ行われるため、ホスト名を変えたら `make deploy-reset`

#### セキュリティの注意

- Keycloak の管理コンソール（`auth-yadori.../admin`）も公開されます。`KC_ADMIN_PASSWORD` は長いものにしてください。
  Cloudflare Access（Zero Trust → Access → Applications）で `auth-yadori.hirohara-keigo.net/admin` に
  メール認証のポリシーを掛けると、自分以外はログイン画面にすら届かなくなります（無料枠で可）
- デモ用のテストアカウント（`operator@example.com` / `password` など）も公開されます。
  デモとして残すなら、Keycloak の管理コンソールでパスワードを変えるか、削除してください
- セルフホストランナーは `main` への push でしか動かず、テストのジョブは GitHub のランナーで走るので、
  フォークからのプルリクエストが自宅サーバーでコードを実行することはありません

---

## ディレクトリ構成

業務領域（文脈）でトップレベルを切り、その内側を実装方式に応じて構成しています。
技術レイヤーでトップレベルを切っていないのは、変更の単位が業務領域であり層ではないためです。

```
yadori/
├── cmd/
│   ├── api/              # HTTPサーバー
│   └── worker/           # 期限切れ回収、決済照合
├── internal/
│   ├── inventory/        # 販売枠の管理（ドメインモデル）
│   │   ├── domain/
│   │   ├── app/
│   │   └── infra/
│   ├── booking/          # 予約管理（ドメインモデル）
│   ├── accommodation/    # 施設情報（アクティブレコード）
│   ├── booker/           # 会員（アクティブレコード）
│   └── search/           # 空室検索（読み取りモデル）
├── db/
│   ├── migrations/
│   └── queries/
└── test/
    └── concurrency/      # 並行テスト
```

構成の不揃いは意図的です。中核領域は層を分け、補完領域はファイルを直に置いています。
補完領域に層を切ったり値オブジェクトを作ったりするのは過剰設計であり、
どこに手をかけるかを分類から判断した結果です。

---

## 用語

| 用語 | 定義 |
|---|---|
| 在庫 | 特定の日・特定の部屋タイプについて、販売可能な枠の数 |
| 確保 | 在庫を一時的に押さえること。期限あり |
| クローズアウト | 宿の判断で、特定の日の販売を停止すること |
| Booker | サイトに登録した利用者アカウント。予約契約の当事者 |
| Guest | 実際に宿泊する人。予約者と異なる場合がある |

業務の言葉で設計し、コードにもその言葉が現れるようにしています。
技術用語を業務領域や集約の名前に使わないという規律を置いています。