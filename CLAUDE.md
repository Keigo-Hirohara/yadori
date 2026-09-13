# yadori 設計ドキュメント

宿泊施設の空室検索・予約を行うWebアプリケーションのバックエンドです。

このドキュメントは**設計判断とその根拠の記録**です。
コードを書く前に必ず読み、記載された方針から逸脱しないでください。
方針を変更する必要が生じた場合は、変更前に理由をこのドキュメントに追記してください。

---

## 0. このリポジトリの位置づけ

転職活動用のポートフォリオです。**単に動くものを作ることが目的ではありません。**

示したいのは以下の3点です。

**① 並行制御を正しく設計・実装できること**

ダブルブッキングを起こさない在庫管理が技術的な主題です。
アプリケーション層のチェックだけでは同時実行を防げないという理解のもと、
DB制約を最終防衛線とする多層防御を設計しています。
100 goroutine による並行テストが、その証明にあたります。

**② 失敗経路を自分で洗い出し、対策を設計できること**

決済は外部サービスのためトランザクションが届きません。
「課金は成立したが予約が確定していない」状態が原理的に避けられないことを踏まえ、
時限付き確保、Webhook との二重経路、定期照合による復旧手段を用意しています。

**③ 設計判断の根拠を説明できること**

ドメイン駆動設計（Vlad Khononov『ドメイン駆動設計をはじめよう』オライリー・ジャパン、2024年）に基づき、
業務領域を分類し、その分類が実装方式の選択を駆動する形で設計しています。
**「なぜそう作ったか」を説明できることが、コードそのものと同等の価値を持ちます。**

このドキュメント自体が成果物の一部です。

---

## 1. 技術スタック

| 領域 | 選定 | 理由 |
|---|---|---|
| 言語 | Go | |
| DB | PostgreSQL 16 | 行ロック、CHECK/UNIQUE制約、部分インデックスが必要 |
| DB層 | sqlc + pgx/v5 | 集約とテーブルを分離できる。SQLを完全に制御できる |
| HTTP | 標準 `net/http` | Go 1.22 以降、ルーティングに十分な機能がある |
| マイグレーション | golang-migrate | |
| テスト | 標準 `testing` + testify + testcontainers | |
| API方式 | REST | 本題は並行制御であり、API方式ではない |

**ORM は使用しません。** ドメインモデルを採用しており、
構造体タグでテーブルと対応させる方式は「集約が永続化を知らない」という方針と衝突します。
また、フィールドを非公開にできないため、集約ルートを唯一の入口とする設計が成立しません。

**GraphQL / gRPC を選ばなかった理由**は、いずれも本題から時間を奪うためです。
GraphQL は N+1 対策やクエリ複雑度制限といった付随課題が増え、
gRPC はブラウザから直接叩けずフロント側の実装が複雑になります。

---

## 2. 業務領域の分類

| 業務領域 | 複雑さ | 差別化 | 分類 | 実装方式 |
|---|---|---|---|---|
| 在庫管理 | 複雑 | しない | **一般**（※） | ドメインモデル |
| 予約管理 | 複雑 | する | **中核** | ドメインモデル |
| 施設情報の管理 | 単純 | しない | **補完** | アクティブレコード |
| 会員管理 | 単純 | しない | **補完** | アクティブレコード |
| 空室検索 | — | — | 在庫管理の読み取り側 | 読み取りモデル（CQRS） |
| 決済・認証 | 複雑 | しない | **一般** | 外部サービス＋腐敗防止層 |

### ※ 在庫管理を中核として自作する理由

**事業としては「一般の業務領域」です。** ダブルブッキングを防ぐことは業界標準であり、
どのOTAも実現しているため差別化になりません。実際の事業なら既製品の採用を検討すべき領域です。

**しかし本プロジェクトでは、意図的に中核として自作します。**
目的が並行制御の設計と実装の実践だからです。

この2つの判断は別の理由に基づくものであり、混同してはいけません。
面接等で説明する際も、事業視点と学習視点を切り分けて述べること。

---

## 3. 区切られた文脈

言葉の意味が変わる場所で境界を引き、4つに分割しています。

```
施設情報の管理   → Accommodation, RoomType
販売枠の管理     → Inventory（Hold を内包）
予約管理         → Booking（Guest を内包）
会員管理         → Booker
```

同じ「部屋タイプ」でも、施設情報では商品の定義、販売枠では日付と結びついた枠を指します。
在庫の「在庫」は宿が売りに出す枠、予約側から見れば単なる確保IDでしかありません。

---

## 4. 業務ルール

実装前に確定させた業務上の判断です。

| 項目 | 決定 |
|---|---|
| 支払い方式 | **前払い**（決済完了をもって予約確定） |
| 仮確保の期限 | **30分** |
| 決済中の上限 | **30分**。超過分は照合処理が拾って決着させる |
| 販売可能数の変更 | **確保済み数を下回る変更は拒否** |
| クローズアウト | `isClosed` フラグ。**枠数を保持したまま新規確保のみ拒否** |
| 部屋タイプの削除 | **論理削除**。既存予約は有効、新規の在庫公開は不可、検索には出さない |
| 宿泊者情報 | **予約時に全員分を必須入力**。後日入力は不可 |
| 宿泊人数 | `guests` の件数から導出。独立したフィールドを持たない |
| 部屋の割り当て | **チェックイン時に宿が行う**。システムは部屋タイプ単位で在庫を管理する |

### キャンセル料規定

| タイミング | 料率 |
|---|---|
| 7日前まで（7日前を含む） | 0% |
| 3〜6日前 | 50% |
| 2日前・前日・当日 | 100% |
| 宿都合・決済失敗・期限切れ | **0%** |

### クローズアウトを `isClosed` にした理由

`quantityAvailable = 0` で表現する案も検討したが、以下の理由で不採用。

- 元の枠数が失われ、販売再開時に復元できない
- 「元々出していない」「売り切れた」「宿が止めた」が区別できない
- 確保済みの枠がある日は、不変条件により 0 に変更できない
  （既存予約を維持したまま新規受付だけ止める運用ができなくなる）

---

## 5. 集約の設計

### 5.1 Inventory（在庫）— 中核

| | |
|---|---|
| 境界 | **部屋タイプ × 日付** の1インスタンス |
| 実装方式 | ドメインモデル |
| 技術方式 | ポートとアダプター + CQRS |
| 不変条件 | **確保済み数 ≤ 販売可能数**（＝ダブルブッキングを起こさない） |

```go
type Inventory struct {
    id                InventoryID   // roomTypeID + date
    holds             []Hold
    fee               Fee
    quantityAvailable int
    isClosed          bool
}

type InventoryID struct {
    roomTypeID uuid.UUID
    date       time.Time   // 日付のみ。時刻は切り捨てる
}

type Hold struct {
    id        uuid.UUID
    bookingID uuid.UUID
    slotNo    int
    status    HoldStatus
    expiresAt *time.Time   // 決済中・確定済みは nil
}

type HoldStatus int
const (
    TemporaryHold HoldStatus = iota
    ProcessingPayment
    Confirmed
)
```

**`Hold` は `inventoryID` を持ちません。** 在庫集約の内部にあるため自明です。
DBの `holds` テーブルには `room_type_id` / `date` がありますが、それは外部キーとして必要なだけで、
mapper が集約の `InventoryID` から補います。

#### メソッド

```go
func Register(id InventoryID, quantity int, fee Fee) (*Inventory, error)

func (inv *Inventory) Hold(holdID, bookingID uuid.UUID, expiresAt, now time.Time) (int, error)
func (inv *Inventory) StartPayment(holdID uuid.UUID) error
func (inv *Inventory) Confirm(holdID uuid.UUID) error
func (inv *Inventory) Release(holdID uuid.UUID) error
func (inv *Inventory) CollectExpired(now time.Time) int

func (inv *Inventory) ChangeQuantity(n int) error
func (inv *Inventory) ChangeFee(f Fee) error
func (inv *Inventory) Close() error
func (inv *Inventory) Reopen() error

func (inv *Inventory) Available() int   // quantityAvailable - len(holds)
func (inv *Inventory) HoldCount() int
func (inv *Inventory) FindHold(holdID uuid.UUID) (Hold, bool)
```

#### この粒度にした理由

**本プロジェクトの中核的な設計判断です。**

- 宿単位にすると、12/24 を予約する人と 3/10 を予約する人が同じロックを奪い合う
- 全期間を1集約にすると、予約が日付をまたぐため芋づる式に肥大化する
  （3泊の予約が3日分の在庫に属し、隣接する予約が次々に連鎖する）
- 部屋タイプ × 日付なら、3泊の予約でも3インスタンスで済み、無関係な日付と競合しない

**集約の境界はロックの粒度でもある**、という理解に基づく判断です。

#### Hold の状態遷移

```
仮確保（期限あり）
  ↓ StartPayment
決済中（expiresAt = nil。期限切れ回収の対象外）
  ↓ Confirm
確定（expiresAt = nil。永続）
```

**「決済中」を設けているのは、決済処理中に期限切れで在庫が解放され、
課金だけが成立する事故を防ぐためです。**
`CollectExpired` は `status == TemporaryHold` かつ `expiresAt` が過去のものだけを対象とします。

### 5.2 Booking（予約）— 中核

| | |
|---|---|
| 境界 | 予約1件 |
| 実装方式 | ドメインモデル |
| 不変条件 | 状態遷移の正しさ、宿泊人数が定員以内、宿泊者が1名以上、宿泊期間の妥当性 |

```go
type Booking struct {
    id         uuid.UUID
    bookerID   uuid.UUID    // 会員集約への参照
    roomTypeID uuid.UUID    // 施設情報集約への参照
    guests     []Guest      // 内部エンティティ
    totalFee   TotalFee
    stayPeriod StayPeriod
    status     Status
}
```

値オブジェクト：`StayPeriod`、`TotalFee`、`Name`。

```go
func Book(id, bookerID, roomTypeID uuid.UUID, period StayPeriod,
          guests []Guest, fee TotalFee, capacity int, now time.Time) (*Booking, error)

func (b *Booking) StartPayment() error
func (b *Booking) Confirm() error
func (b *Booking) Cancel(reason CancellationReason, now time.Time) (CancellationFee, error)
func (b *Booking) ChangeGuests(guests []Guest, capacity int) error
func (b *Booking) GuestCount() int   // len(guests)
```

#### 定員チェックは値を受け取って行う

`Book` と `ChangeGuests` は `capacity int` を引数で受け取ります。
**予約集約が施設情報集約に依存しないため**です。
アプリケーション層が RoomType から取得して渡します。

#### 確定料金は予約時点でコピーする

料金プランを参照し続けてはいけません。
宿が後から値上げした場合に既存予約の金額が変わるのは、業務上も法的にも許されません。
値オブジェクト `TotalFee` として予約集約内に固定します。

#### StayPeriod.Dates()

在庫を確保する対象日を導出します。3泊なら3件、**必ず昇順**で返します。
**昇順であることがデッドロック回避の前提**なので、テストで明示的に検証しています。

#### 確保への参照

予約集約は `holdIDs` を保持しません。
確保は在庫集約に属し、DBでは `holds.booking_id` で紐づきます。
補償に必要な確保IDは**プロセスマネージャーが記録します**。

### 5.3 Accommodation / RoomType（施設情報）— 補完

アクティブレコード。層を切らず、型・検証・永続化が同居します。

```go
type Accommodation struct {
    id            uuid.UUID
    name          string
    phoneNumber   string   // ハイフン除去済み
    postalCode    string   // ハイフン除去済み
    prefecture    string
    city          string
    streetAddress string
    building      string
}
```

| フィールド | 検証 |
|---|---|
| Name | trim 後 1〜60文字（`utf8.RuneCountInString`） |
| PhoneNumber | ハイフン除去後 `^0\d{9,10}$` |
| PostalCode | ハイフン除去後 `^\d{7}$` |
| Prefecture / City / StreetAddress | 空でない |
| Building | 空を許容 |

`RoomType` は `capacity >= 1` を厳格に検証します。
**補完領域だが、他の集約が依存する値の検証は省略しないこと。**
論理削除（`deletedAt`）を持ちます。

### 5.4 Booker（会員）— 補完

アクティブレコード。氏名と電話番号が必須、住所は `DEFAULT ''`。

**住所は現在使用しません。** 将来、予約時の宿泊者情報入力を省略する用途を想定して
フィールドとしては保持しますが、現時点では参照しません。

「予約者としての役割」も同じ `Booker` で表します（`Booking.bookerID` が参照）。

---

## 6. ダブルブッキング防止

### 多層防御

| 層 | 手段 |
|---|---|
| ドメインモデル | `len(holds) <= quantityAvailable` を検証。空き枠番号の割り当て |
| トランザクション | `SELECT ... FOR UPDATE` による行ロック |
| **DB制約** | **`UNIQUE (room_type_id, date, slot_no)`** |

**アプリケーション層の検証だけでは同時実行を防げません。最終防衛線をDBに置きます。**

### 枠番号（slot_no）方式

各確保に 1〜quantityAvailable の枠番号を割り当て、一意制約で二重取得を防ぎます。

**枠番号を決めるのはドメインモデルです。**

```go
func (inv *Inventory) findFreeSlot() (int, bool) {
    used := make(map[int]bool, len(inv.holds))
    for _, h := range inv.holds {
        used[h.slotNo] = true
    }
    for i := 1; i <= inv.quantityAvailable; i++ {
        if !used[i] { return i, true }
    }
    return 0, false
}
```

**DBに決めさせる案（`generate_series` を使う INSERT）は不採用です。**
理由は、空き枠を探すことが業務ロジックであり、集約が持つべきだからです。
また、DBに任せると集約のテストがDBなしで完結しません。

`SELECT FOR UPDATE` でロックを取るため実際には衝突しませんが、
`UNIQUE` 制約は最終防衛線として残します。

**解放された枠番号は再利用されます。** 枠番号に業務上の意味はなく、
実際の部屋割り当てはチェックイン時に宿が行うためです。

### デッドロック回避

複数日をまたぐ在庫のロックは、**必ず日付の昇順**で取得します。
`StayPeriod.Dates()` が昇順を保証しているため、そのままループすれば守られます。

### 検証するテスト

```
・残1枠に100 goroutine が同時に確保 → 成功が正確に1件、DBの行数も1
・3日分の在庫を複数プロセスが同時に取り合う → デッドロックが発生しない
・quantityAvailable を確保数より小さくしようとする → 拒否される
```

`go test -race` を必ず使用します。

---

## 7. 結果整合性と処理順序

在庫と予約は別の集約であり、**1トランザクション1集約の原則を守ります。**

### 処理順序

`holds.booking_id` に NOT NULL の外部キーがあるため、予約を先に作ります。

```
T1: 予約を作成（status = temporary_hold）
T2: 12/24 の在庫を読み、確保を追加
T3: 12/25 の在庫を読み、確保を追加
T4: 12/26 の在庫を読み、確保を追加
    （外部システム）決済
T5〜T7: 各在庫の確保を確定（expiresAt を nil に）
T8: 予約を確定
```

**各トランザクションが1集約のみを更新しています。**

守るべき規律は2つです。

1. **確保が先、予約の確定が後。** 確定した予約の在庫が存在しない状態を防ぐ
2. **確保の確定（T5〜T7）が、予約の確定（T8）より先。** 逆にすると、確定済み予約の在庫が期限切れで消える

### 中間状態の許容

「在庫だけ確保されて予約が確定していない」状態は**業務的に壊れていません。**
「誰かが予約手続き中」という正常な状態であり、期限（30分）で必ず解消されます。

途中で失敗した場合はプロセスマネージャーが補償（解放）しますが、
**補償が失敗しても致命的ではありません。** 期限切れ回収バッチが後で拾います。二重の安全網です。

### 検討したが採用しなかった案

**在庫3件＋予約1件を1トランザクションで更新する案**（1トランザクション1集約からの逸脱）。

同一DBに収まっており技術的には可能で、実装も単純です。実務ではこちらが妥当な場面も多い。

採用しなかったのは、本プロジェクトが原則に忠実な設計の実践を目的としているためです。
また、この方式では9章（サーガ、プロセスマネージャー、アウトボックス）を実践する機会がなくなります。

---

## 8. 決済連携

外部サービスのため DB トランザクションが届きません。
**「片方だけ完了している」状態が原理的に避けられないため、復旧手段を用意します。**

### 対策

**① プロセスマネージャーが状態をDBに保持する**

各ステップの完了後に必ず保存します。どこまで進んだかが記録されるため、中断地点から再開できます。
`holdIDs`（確保ID + 在庫ID）を保持し、補償時に何を解放すべきか分かるようにします。

**② 同期レスポンスと Webhook の二重経路**

片方を取りこぼしても、もう片方で確定処理が実行されます。
両方届く可能性があるため、**受信処理は冪等**に実装します（決済IDで処理済みを判定）。

**③ 定期的な照合**

「決済依頼済みだが確定していない」予約を検出し、決済サービスに実際の課金状況を問い合わせます。
課金済みなら予約を確定（救済）、未課金なら在庫を解放します。

### 腐敗防止層

決済サービスのレスポンスをそのまま業務ロジックに渡しません。
自分たちの言葉（`決済結果`）に変換する層を挟み、外部の仕様変更が業務ロジックに波及しないようにします。

### 実装方針

**モックの決済サービスで構いません。** 「成功」「失敗」「無応答」を切り替えられるようにすれば、
むしろ本物より異常系のテストが書きやすくなります。
ポートフォリオとしての価値は、プロセスマネージャーと照合処理が動くことにあります。

---

## 9. 空室検索（CQRS）

### 分ける理由

在庫集約は「部屋タイプ × 日付」の粒度であり、
「千葉県で12/24から2泊、大人2名、15,000円以下」という検索に答えられません。
数万件の集約をメモリに載せることになります。

**書き込み側と読み取り側で要求が正反対**のため、モデルを分離します。

**CQRS はイベント履歴式（イベントソーシング）とは独立した技術方式です。**
本プロジェクトはイベント履歴式を採用しませんが、CQRS は採用します。

### 読み取りモデル

**部屋タイプ単位の行**とします。宿名・住所・定員などを非正規化し、結合なしで検索できる形にします。
正規化の原則にあえて反していますが、読み取り専用モデルではこれが正しい形です。

連泊の判定は `HAVING COUNT(*) = 泊数` で行います（全日程に空きがある部屋タイプだけが残る）。

### 投影

**同期投影**とし、在庫の更新と同じトランザクションで検索テーブルを更新します。

**投影の呼び出しはリポジトリの `Save` の中の一箇所のみ**とします。
各コマンドに投影処理を書くと、コマンドが増えたときに書き忘れが発生します。
在庫を変えるコマンドは7つ以上あり、人間の注意力では守れません。

検索テーブルは在庫集約から再構築可能な派生データです。
不整合に備え、**全削除して作り直す処理を用意します。**

### 業務上の論点

投影にタイムラグがある場合、「検索では空室と出たが予約時には満室」が起きます。
これは欠陥ではなく、実際のOTAで日常的に起きていることです。
**検索結果は参考情報であり、確定は予約時**という業務ルールになります。
予約時には必ず在庫集約で再確認します。

---

## 10. DBスキーマ

集約とテーブルは1対1ではありません。集約は読み書きの単位、テーブルは保存の形です。
値オブジェクトは親テーブルのカラムに展開し、別テーブルにしません。

```sql
CREATE TABLE accommodations (
    id             UUID PRIMARY KEY,
    name           TEXT NOT NULL,
    phone_number   TEXT NOT NULL,
    postal_code    TEXT NOT NULL,
    prefecture     TEXT NOT NULL,
    city           TEXT NOT NULL,
    street_address TEXT NOT NULL,
    building       TEXT NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE room_types (
    id               UUID PRIMARY KEY,
    accommodation_id UUID NOT NULL REFERENCES accommodations(id),
    name             TEXT NOT NULL,
    capacity         INT  NOT NULL CHECK (capacity >= 1),
    has_private_bath BOOLEAN NOT NULL DEFAULT false,
    has_balcony      BOOLEAN NOT NULL DEFAULT false,
    deleted_at       TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE bookers (
    id             UUID PRIMARY KEY,
    first_name     TEXT NOT NULL,
    last_name      TEXT NOT NULL,
    phone_number   TEXT NOT NULL,
    postal_code    TEXT NOT NULL DEFAULT '',
    prefecture     TEXT NOT NULL DEFAULT '',
    city           TEXT NOT NULL DEFAULT '',
    street_address TEXT NOT NULL DEFAULT '',
    building       TEXT NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 集約の識別子をそのまま主キーにする（代理キーを持たない）
CREATE TABLE inventories (
    room_type_id       UUID NOT NULL REFERENCES room_types(id),
    date               DATE NOT NULL,
    fee                INT  NOT NULL CHECK (fee >= 0),
    quantity_available INT  NOT NULL CHECK (quantity_available >= 0),
    is_closed          BOOLEAN NOT NULL DEFAULT false,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (room_type_id, date)
);

CREATE TYPE booking_status AS ENUM (
    'temporary_hold', 'processing_payment', 'confirmed', 'cancelled'
);

CREATE TABLE bookings (
    id            UUID PRIMARY KEY,
    booker_id     UUID NOT NULL REFERENCES bookers(id),
    room_type_id  UUID NOT NULL REFERENCES room_types(id),
    total_fee     INT  NOT NULL CHECK (total_fee >= 0),
    checkin_date  DATE NOT NULL,
    checkout_date DATE NOT NULL,
    status        booking_status NOT NULL DEFAULT 'temporary_hold',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT valid_period CHECK (checkout_date > checkin_date)
);

CREATE TYPE hold_status AS ENUM (
    'temporary_hold', 'processing_payment', 'confirmed'
);

CREATE TABLE holds (
    id           UUID PRIMARY KEY,
    room_type_id UUID NOT NULL,
    date         DATE NOT NULL,
    booking_id   UUID NOT NULL REFERENCES bookings(id),
    slot_no      INT  NOT NULL CHECK (slot_no >= 1),
    status       hold_status NOT NULL DEFAULT 'temporary_hold',
    expired_at   TIMESTAMPTZ,             -- 決済中・確定済みは NULL
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    FOREIGN KEY (room_type_id, date) REFERENCES inventories (room_type_id, date),
    CONSTRAINT uq_slot UNIQUE (room_type_id, date, slot_no)
);

CREATE TABLE guests (
    id         UUID PRIMARY KEY,
    booking_id UUID NOT NULL REFERENCES bookings(id) ON DELETE CASCADE,
    first_name TEXT NOT NULL,
    last_name  TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

**`guests` の `ON DELETE CASCADE`** は、宿泊者が予約集約の内部であることの表現です。

**インデックスは意図的にまだ貼っていません。** 負荷試験の段階で、
計測 → 特定 → 追加 → 再計測 の往復を記録に残すためです。
貼る候補は `holds(expired_at) WHERE status='temporary_hold'` の部分インデックスなど。

---

## 11. HTTP API

### 11.1 HTTPハンドラは駆動アダプター

ハンドラは**駆動アダプター（プライマリアダプター）**です。
`infra/postgres` が「アプリ→DB」を変換するのと対称の位置にいます。

責務は3つだけです。

1. HTTPリクエスト → ユースケースの入力に変換する
2. ユースケースを呼ぶ
3. 結果とエラー → HTTPレスポンスに変換する

**書いてはいけないもの**は、業務判断・トランザクション管理・SQL です。
判断基準は「**このハンドラを gRPC やCLIに置き換えたとき、消えてなくなるコードか**」。
消えないものが混じっていたら、それは `app` か `domain` に属します。

### 11.2 駆動ポートはハンドラ側に、狭く定義する

**インターフェースは `app` ではなく `infra/http` に置きます。**

```go
// internal/inventory/infra/http/handler.go
type inventoryService interface {
    Publish(ctx context.Context, in app.PublishInput) error
    Close(ctx context.Context, roomTypeId uuid.UUID, date time.Time) error
    // このハンドラが使う操作だけを宣言する
}
```

`app` 側には何も足しません。`*app.Service` は何も知らないまま、
Go の暗黙実装でこれを満たします。

#### この判断の理由

**駆動側に依存性逆転は必要ありません。** DIP が禁じているのは
「方針が詳細に依存する」状態ですが、駆動側は `infra/http`（詳細）→ `app`（方針）と、
最初から向きが正しい。逆転させるものがありません。
逆転が必要なのは被駆動側（`app` → `infra/postgres` になってしまう側）だけです。

したがって駆動側にインターフェースを置くのは、DIP の適用ではなく
**差し替え可能性のための抽象化**です。目的が違うので、置き方も変わります。

**ハンドラ側に置くのは、14章の「インターフェースは利用側で定義する」に従った結果です。**
駆動ポートでは2つの原則が衝突します。

| 原則 | 被駆動ポート | 駆動ポート |
|---|---|---|
| ポートはアプリケーションのもの | `app` に置く | **`app` に置く** |
| インターフェースは利用側で定義する | 利用側＝`app` | **利用側＝ハンドラ** |

被駆動側では両方が同じ答えを出すので迷いません。駆動側でだけ答えが割れます。
本プロジェクトは後者を採り、`Repository` を `domain` から `app` に移したときと
同じ基準で判断しています。

狭く定義することで、`app` にメソッドを足しても使わない限り書き足す場所が増えません。

### 11.3 ディレクトリ

```
internal/inventory/infra/http/     在庫API
internal/booking/infra/http/       予約API
internal/shared/http/              ミドルウェア、共通のレスポンス形式
cmd/api/                           ルーティングと組み立て（合成の根）
```

`cmd/api/main.go` が唯一、全部を知っている場所です。
pgxpool を作り、Transactor を作り、Service を作り、ハンドラに渡し、ルータに登録します。

### 11.4 バリデーションは構文だけ

| 層 | 見るもの | 責務 |
|---|---|---|
| **構文** | JSONが壊れていない、UUIDとして読める、日付形式 | **アダプター** |
| **値の妥当性** | チェックアウト > チェックイン、料金が非負 | **ドメイン** |
| **業務ルール** | 満室でない、定員以内、遷移が正しい | **ドメイン** |

**アダプターに値の妥当性を書きません。** 二重管理になり、
「集約のテストが業務ルールの仕様書」（16章）という立場が崩れるためです。
ドメインに投げて、返ってきた番兵エラーをコードに写します。

フィールド単位のエラーは返しません。**フロントエンドで別途バリデーションを行い、
どのフィールドが不正かはそちらで判定する**前提です。
サーバー側の構文チェックは、壊れたリクエストで500を出さないための防波堤として機能します。

### 11.5 エラーコードとステータスの写像

レスポンスの形。

```json
{ "code": "INVENTORY_SOLD_OUT", "message": "空きがないため確保できません" }
```

写像は**文脈ごとに1ファイル**（`infra/http/error.go`）にまとめます。
各ハンドラに `errors.Is` を散らすと、エラーが増えたときに書き漏らします。
番兵エラーは `%w` でラップされうるので、map ではなく `errors.Is` で走査します。

| エラー | コード | ステータス |
|---|---|---|
| `ErrInventoryNotFound` | `INVENTORY_NOT_FOUND` | 404 |
| `ErrBookingNotFound` | `BOOKING_NOT_FOUND` | 404 |
| `ErrHoldNotFound` | `HOLD_NOT_FOUND` | 404 |
| `ErrSoldOut` | `INVENTORY_SOLD_OUT` | **409** |
| `ErrClosed` | `INVENTORY_CLOSED` | 409 |
| `ErrAlreadyRegistered` | `INVENTORY_ALREADY_REGISTERED` | 409 |
| `ErrDuplicatedHold` | `HOLD_DUPLICATED` | 409 |
| `ErrInvalidTransition` | `INVALID_TRANSITION` | 409 |
| `ErrQuantityBelowHolds` | `QUANTITY_BELOW_HOLDS` | 409 |
| `ErrOverCapacity` | `OVER_CAPACITY` | 422 |
| `ErrNoGuest` | `NO_GUEST` | 422 |
| `ErrInvalidQuantity` | `INVALID_QUANTITY` | 400 |
| `ErrMinusFee` | `INVALID_FEE` | 400 |
| `ErrPast` | `PAST_DATE` | 400 |
| `ErrInvalidStayPeriod` | `INVALID_STAY_PERIOD` | 400 |
| `ErrBusy`（ロック待ち超過） | `RESOURCE_BUSY` | 503 + `Retry-After` |
| 構文エラー | `INVALID_REQUEST_BODY` | 400 |
| それ以外 | `INTERNAL_ERROR` | 500 |

**満室を 409 にする理由。** 400 は「あなたのリクエストが間違っている」、
409 は「リクエストは正しいが、いまの状態と衝突した」。満室は後者です。
同じリクエストを明日投げれば通るかもしれません。

**500 では `err.Error()` を返しません。** 内部情報が漏れるため、ログにだけ出します。

#### コードの規約

- 大文字スネーク、必要なら文脈の接頭辞を付ける（`INVENTORY_` / `BOOKING_`）
- **コードは API の契約。** 番兵エラーの変数名やメッセージを変えてもコードは変えない
- `message` は番兵エラーの日本語をそのまま返す。フロントはコードで分岐し、
  `message` はフォールバック表示に使う

### 11.6 ミドルウェア

`internal/shared/http/` に置き、`cmd/api` で積みます。

| ミドルウェア | 目的 |
|---|---|
| Recover | パニックでプロセスが落ちるのを防ぐ |
| RequestID | 障害調査。context とレスポンスヘッダの両方に載せる |
| AccessLog | 同上 |
| Timeout | 下記 |
| MaxBytes | `http.MaxBytesReader`。巨大なJSONでメモリを食われるのを防ぐ |

認証・CORS・レート制限は後回しです。

#### タイムアウトは2段構え（本プロジェクトの主題と直結）

`SELECT ... FOR UPDATE` のロック待ちには、PostgreSQL のデフォルトで上限がありません。
同じ在庫にリクエストが集中したとき、待っている接続が pgx のコネクションプールを
食い尽くし、**無関係なAPIまで応答しなくなります。**

**① リクエスト単位の context タイムアウト**（ミドルウェア、暫定 5秒）

pgx は context のキャンセルを尊重するので、待っているクエリが中断されます。

**② ロック待ちの上限**（`Transactor.WithinTx` の中、暫定 3秒）

```sql
SET LOCAL lock_timeout = '3s';
```

ロックが取れないとき、待ち続けずに即座に失敗します（エラーコード `55P03`）。

これを**インフラの言葉のままハンドラに渡さず**、`app` 層の番兵エラー `ErrBusy` に
翻訳します。外部キー違反を業務の言葉に翻訳している `roomType.go` と同じ作法です。

`ErrBusy` は業務ルールではなく運用上の状態なので、`inventory/app/port.go` に置きます。

**秒数（5秒 / 3秒）は暫定値です。** 負荷試験（13章の段階10）で調整します。

### 11.7 エンドポイント一覧

前置きは `/api/v1`。宿側は `/api/v1/admin` に束ねます。

**サーバーは1つです。** 管理者用サイトと宿泊者用サイトでフロントエンドは分かれますが、
APIを物理的に分ける実利はこの規模ではありません。
接頭辞だけ分ける理由は、**ルータでプレフィックスごとに認可をかけられる**ためです。
単一の口に混在させると、管理エンドポイントを足したときに認可の付け忘れが
そのまま脆弱性になります。接頭辞のコストはほぼゼロで、後から入れるのは破壊的変更です。

将来 `cmd/admin-api` として切り出す余地も残ります。

#### 宿側 `/api/v1/admin`

| メソッド | パス | 成功 | 主なエラーコード |
|---|---|---|---|
| `POST` | `/accommodations` | 201 | `INVALID_REQUEST_BODY` |
| `GET` | `/accommodations/{accommodationId}` | 200 | `ACCOMMODATION_NOT_FOUND` |
| `POST` | `/accommodations/{accommodationId}/room-types` | 201 | `INVALID_CAPACITY` |
| `GET` | `/room-types/{roomTypeId}` | 200 | `ROOM_TYPE_NOT_FOUND` |
| `POST` | `/room-types/{roomTypeId}/inventories` | 201 | `INVENTORY_ALREADY_REGISTERED` 409 / `PAST_DATE` 400 |
| `PUT` | `/room-types/{roomTypeId}/inventories/{date}/quantity` | 204 | `QUANTITY_BELOW_HOLDS` 409 / `INVALID_QUANTITY` 400 |
| `PUT` | `/room-types/{roomTypeId}/inventories/{date}/fee` | 204 | `INVALID_FEE` 400 |
| `POST` | `/room-types/{roomTypeId}/inventories/{date}/close` | 204 | `INVENTORY_CLOSED` 409 |
| `POST` | `/room-types/{roomTypeId}/inventories/{date}/reopen` | 204 | `INVENTORY_NOT_CLOSED` 409 |
| `GET` | `/room-types/{roomTypeId}/inventories?from=&to=` | 200 | — |

**操作ごとにURLを分けます。** ドメインが `ChangeQuantity` / `ChangeFee` / `Close` を
別の業務操作として持っているためです。1つの `PATCH` にまとめると、
どのフィールドが来たかでハンドラが分岐することになり、判断がアダプターに漏れます。

**在庫の一括公開APIは作りません。** 複数日＝複数集約であり、
1トランザクションにまとめると7章の原則を破ります。
フロントエンドが日数ぶん並列に `POST` を投げます。
既に登録済みの日は 409 が返るので、フロント側で `PUT .../quantity` に切り替えます。

#### 宿泊者側 `/api/v1`

| メソッド | パス | 成功 | 主なエラーコード |
|---|---|---|---|
| `GET` | `/room-types/search?prefecture=&checkin=&checkout=&guests=&max_fee=` | 200 | `INVALID_REQUEST_BODY` 400 |
| `POST` | `/bookers` | 201 | `INVALID_REQUEST_BODY` 400 |
| `POST` | `/bookings` | 201 | `INVENTORY_SOLD_OUT` 409 / `OVER_CAPACITY` 422 / `NO_GUEST` 422 / `RESOURCE_BUSY` 503 |
| `GET` | `/bookings/{bookingId}` | 200 | `BOOKING_NOT_FOUND` 404 |
| `POST` | `/bookings/{bookingId}/payment` | 200 | `INVALID_TRANSITION` 409 / `PAYMENT_FAILED` 402 |
| `POST` | `/bookings/{bookingId}/cancel` | 200 | `BOOKING_NOT_FOUND` 404 |
| `PUT` | `/bookings/{bookingId}/guests` | 204 | `OVER_CAPACITY` 422 / `INVALID_TRANSITION` 409 |

`cancel` はキャンセル料を返します。

```json
{ "bookingId": "...", "status": "cancelled", "cancellationFee": 15000 }
```

#### 暫定の決済エンドポイント

```json
POST /api/v1/bookings/{bookingId}/payment
{ "result": "success" }        // "failure" / "timeout" も受ける
```

いまは中で `booking/app.Confirm` を呼ぶだけです。
結果を切り替えられる形にしておくと、8章のモック決済にそのまま育ちます。

**このエンドポイントは決済サービスを繋いだ時点で消えます。**
本来の確定は決済のコールバックと Webhook の二重経路（8章）から入ります。

#### APIに出さないもの

- `Hold` / `StartPayment` / `Confirm` / `Release`
  … 予約のユースケース経由でしか呼ばれない
- `CollectExpired` … ワーカー（`cmd/worker`）の担当

### 11.8 既知の宿題

- **`BookInput.BookerId` をリクエストボディで受けている。**
  このままでは他人になりすまして予約できる。認証を入れる際に、
  必ず context 由来（トークンから取り出した値）に変えること

---

## 12. 実装の現状

### 完了

| 領域 | 状態 |
|---|---|
| イベントストーミング | 完了（業務イベント・コマンド・アクター・読み取りモデル） |
| 設計（集約・文脈・実装方式） | 完了 |
| マイグレーション | 完了 |
| sqlc のクエリ定義 | 在庫・予約・施設情報・会員 |
| `accommodation` | 型・検証・`Save`・`FindByID` |
| `room_type` | 型・検証 |
| `booker` | 型・検証 |
| `inventory/domain` | `Fee`、`InventoryID`、`Hold`、`Inventory` 一式とテスト |
| `booking/domain` | `TotalFee`、`StayPeriod`、`Name`、`Guest`、`Booking` とテスト |

### 未着手

- リポジトリの実装（`infra/postgres`）
- **並行テスト**
- アプリケーション層（在庫と予約の連携、プロセスマネージャー）
- REST API
- 決済連携
- 空室検索と投影
- 期限切れ回収ワーカー
- 負荷試験

### 既知の未完了項目

- **`Inventory.Hold` の過去日付検証が未実装。** `now` を引数で受け取り、
  `inv.id.Date()` と比較して過去なら拒否する方針
- **`Accommodation` / `RoomType` の ID 生成が集約内部で行われている。**
  方針としてはアプリケーション層で生成に統一したいが、未修正
- `Booking` の状態が4つのみ（`staying` / `completed` / `no_show` は未定義）
- `guests` の年齢を持たない（用途がないため削除済み）

---

## 13. これからの実装順序

**動くものを早く作ることを優先します。** 設計を完璧にしてから実装するのではありません。

```
1. Repository の実装（在庫 → 予約）
2. 並行テスト                          ← 最優先。本プロジェクトの中核
3. 空室検索（暫定：在庫を直接引く単純版でよい）
4. アプリケーション層（予約 → 在庫確保 → 補償）
5. REST API
6. フロントの繋ぎ込み
──── ここまでで動くものになる ────
7. 決済（モック）+ プロセスマネージャー
8. 期限切れ回収ワーカー
9. CQRS への置き換え（投影、再構築処理）
10. 負荷試験とプロファイリング
```

### 2 を前倒しする理由

**API より先に並行テストを書いてください。**

- このプロジェクトの主題であり、最後に回すと時間切れの危険がある
- リポジトリの設計（`FOR UPDATE` の位置、ロック順序）を検証できる。
  破れていた場合、API 実装後だと手戻りが大きい
- API 層は並行制御に関係しない。ハンドラを挟んでも防御の仕組みは変わらない

並行テストはリポジトリと集約だけで書けます。サービス層も不要です。

### 10 の進め方

**目標値を先に決めてください。**（例：予約APIの p99 が 200ms 以内）

pprof でボトルネックを特定 → 改善 → 再計測。
**この一往復があるかどうかで評価が変わります。**
「負荷試験をやりました」ではなく「N倍改善しました、原因はこれでした」と言える形にすること。

---

## 14. ディレクトリ構成

業務領域（文脈）でトップレベルを切り、その内側を実装方式に応じて構成します。
**技術レイヤーでトップレベルを切りません。** 変更の単位は業務領域であり、層ではないためです。

```
yadori/
├── cmd/
│   ├── api/              # HTTPサーバー
│   └── worker/           # 期限切れ回収、決済照合
├── internal/
│   ├── inventory/        # 中核・ドメインモデル
│   │   ├── domain/       # 業務ロジック層（集約・値オブジェクト）
│   │   ├── app/          # アプリケーション層（ユースケース・Repositoryなどのインターフェース）
│   │   └── infra/        # インフラストラクチャ層
│   │       ├── postgres/ # Repository実装・mapper
│   │       └── http/
│   ├── booking/          # 中核・ドメインモデル（同構成）
│   ├── accommodation/    # 補完・アクティブレコード（層を切らない）
│   ├── booker/           # 補完・アクティブレコード
│   ├── search/           # 読み取りモデル
│   └── shared/
│       ├── db/           # コネクション、トランザクション
│       └── payment/      # 決済の腐敗防止層
├── db/
│   ├── migrations/
│   └── queries/          # sqlc用SQL（文脈ごとにサブディレクトリ）
└── test/
    └── concurrency/      # 並行テスト
```

**構成の不揃いは意図的です。** 中核は層を分け、補完はファイルを直に置いています。
補完領域に層を切ったり値オブジェクトを作ったりするのは過剰設計であり、
どこに手をかけるかを分類から判断した結果です。

### 依存の向き

```
infra → app → domain
```

インターフェースは**利用側**で定義し、実装を `infra` に置きます。
**`domain` は `infra` を import しません。**
Go の循環 import 禁止により、これは言語レベルで保証されます。

### Repository を `app` に置く理由

`Repository` と `Transactor` は `domain` ではなく **`app` に定義します。**

古典的な DDD では、集約のコレクション抽象としてリポジトリをドメイン層に置きます。
それを採らなかった理由は2つです。

- **利用しているのは `app` だから。** 本プロジェクトにドメインサービスはなく、
  集約は自分だけで不変条件を守れています。`domain` はリポジトリを一度も呼びません。
  「インターフェースは利用側で定義する」を字義どおり適用すると、置き場所は `app` になります
- **`FindForUpdate` と `Transactor` は永続化の機構だから。** 行ロックとトランザクション境界は
  業務に対応物がありません。これらが `domain` にあると、15章の
  「集約は永続化を知らない」が名目だけのものになります

`ErrInventoryNotFound` のような番兵エラーは業務の言葉なので `domain` に残します。

---

## 15. 実装規約

### ドメインモデル（inventory, booking）

- **フィールドは非公開。** 集約ルートのメソッド経由でのみ操作する
- 集約は**永続化を知らない。** SQL も sqlc 生成物も domain 層に登場させない
- 集約をまたぐ参照は**IDで行う。** 他集約の実体を保持しない
- 値オブジェクトは**イミュータブル、IDを持たない**
- 状態を変える操作は、単純に見えても**必ず集約のメソッド経由**とする
  （参照のみ集約を経由しなくてよい）
- **現在時刻・IDは引数で受け取る。** 集約が `time.Now()` や `uuid.New()` を呼ばない
- 内部エンティティ（`Hold`）は集約が生成する。外部から直接作らせない
- DBからの復元専用関数は検証を通さない（保存時に検証済みのため。
  検証を厳しくしたときに既存データが読めなくなるのを防ぐ）

### アクティブレコード（accommodation, booker）

- **値オブジェクトを作らない。** 住所はフラットなフィールドとして持つ
- **リポジトリインターフェースを作らない**
- **レイヤー分けをしない**
- **ドメインイベントを発行しない**
- **依存性注入をしない。** DB接続は `ctx, db` を引数で受け取る
  （`ctx, db` を引数に取るメソッドは DB に触る、という規約）
- フィールドは非公開、参照はゲッター経由
- **更新は新しいインスタンスを作り直す**（セッターを作らない）
- 検証はファクトリ関数 `New` の中で行う
- **入力用の構造体のフィールドは公開する**（パッケージ外から呼べる必要があるため）
- 1パッケージに複数の型がある場合、`NewAccommodation` のように型名を含める

補完領域だが、**他の集約が依存する値（`RoomType.capacity`）の検証は省略しない。**

### エラー

**番兵エラー**（`var ErrXxx = errors.New(...)`）を定義し、`errors.Is` で判定可能にします。
`fmt.Errorf` で都度生成すると呼び出し側が種類を判定できず、
HTTP ハンドラでステータスコードを出し分けられません。

文脈を足す場合は `fmt.Errorf("...: %w", err)` でラップします。

### 命名

- 業務領域・文脈・集約の名前 → **業務の言葉**
- 集約のメソッド名 → **業務の言葉**（`Hold`、`Close`、`Confirm`）
- リポジトリ、投影、プロセスマネージャー → 技術の言葉でよい（業務に対応物がないため）
- 型名・関数名・フィールド名は英語、エラーメッセージは日本語
- ファイル名はスネークケース

**技術用語を業務の名前に使わないこと。**
例：「在庫の一貫性」は技術用語なので「在庫管理」とする。

### 用語集

| 用語 | 定義 |
|---|---|
| 在庫（Inventory） | 特定の日・特定の部屋タイプについて、販売可能な枠の数 |
| 登録（Register） | 宿が、特定の日・特定の部屋タイプについて、枠数と料金を決めて売りに出すこと。一度だけ行う |
| 確保（Hold） | 在庫を一時的に押さえること。期限あり |
| 枠番号（slot_no） | 在庫1枠を識別する 1〜quantityAvailable の番号。業務上の意味はない |
| クローズアウト | 宿の判断で、特定の日の販売を停止すること |
| Booker | サイトに登録した利用者アカウント。予約契約の当事者 |
| Guest | 実際に宿泊する人。予約者と異なる場合がある |

---

## 16. テスト方針

| 対象 | テストの種類 |
|---|---|
| 中核のドメインモデル | 単体テスト（DBなし）。業務ルールを検証 |
| アクティブレコード | 検証ロジックは DBなし、永続化は testcontainers |
| リポジトリ | testcontainers で本物の PostgreSQL を使う |
| **ダブルブッキング** | **並行実行テスト（必須）** |

### 集約のテストは業務ルールの仕様書

サブテスト名を業務の言葉で書き、テスト結果の出力がそのまま業務ルールの一覧になるようにします。

```
--- PASS: TestInventory_Hold/空きがあれば確保できる
--- PASS: TestInventory_Hold/満室のときは確保できない
--- PASS: TestInventory_CollectExpired/決済中の確保は回収されない
```

### DRY の適用範囲

- **検証している内容**（何を確かめているか）→ **DRY にしない**
- **セットアップ**（テスト対象を用意する手順）→ **DRY にする**

判断基準は「そのコードを消したら、何をテストしているか分からなくなるか」。

### テスト実行

```bash
go test -race -count=1 ./...   # 全テスト
go test -short ./...            # DB不要な高速テストのみ
```

並行制御の検証のため `-race` を常用します。

---

## 17. これからの展望

### 料金プランの分離（11章「設計を進化させる」の実践）

現在、料金は在庫集約の一部として保持しています（1部屋タイプ = 1料金）。

食事付きプラン、連泊割引、曜日別料金といった要求が生じた場合、
料金は独立したライフサイクルと複雑なルールを持つようになります。
その時点で**補完 → 中核へ格上げし、料金プランを独立した集約として切り出します。**

**分割の理由が業務要求として記録される**のがこの進め方の価値です。
最初から分けていたら「なぜ分けたのか」は設計者の勘としか言えません。

分割に備え、料金の計算ロジックは値オブジェクトとして切り出しておきます。

### スケール戦略

在庫集約が「部屋タイプ × 日付」という小さな単位であり、
他の集約と強い整合性を要求しない設計のため、**宿単位での水平分割が可能です。**

書き込みが逼迫した場合は、読み取りレプリカの分離 → 宿単位のシャーディング、の順で対応できます。
NewSQL（CockroachDB 等）も選択肢ですが、分散トランザクションのレイテンシと運用コストが増えるため、
この規模では不要です。

**集約の境界設計が、将来のスケール戦略を規定しています。**
在庫と予約を1つの巨大な集約にしていたら、分割できませんでした。

### 分類の変化への対応

| 変化 | 対応 |
|---|---|
| 補完 → 中核 | **作り直す。** アクティブレコードでは業務ルールを守れなくなるため |
| 中核 → 補完 | **作り直さない。** 新機能の追加を止め、投資を減らすだけ |
| 中核 → 一般 | 既製品への置き換えを検討する |

分類が変わったときに変わるのは、主に「これから何に手をかけるか」であり、
既存コードの構造ではありません。