# yadori 設計ドキュメント

宿泊施設の空室検索・予約を行うWebアプリケーションです。設計の主題はバックエンドにあります。

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
| フロントエンド | Vite + React + TypeScript + Tailwind | 管理画面と予約者向けサイトの2アプリ。本題ではないので最小構成 |
| 起動 | Makefile + Docker Compose | `make dev` でAPI・2つのフロント・を一括起動 |

**sqlc は v1.30 以降を使います。** v1.27 は macOS の新しい SDK で `pg_query_go` のビルドが通りません。
`make sqlc` が `go run github.com/sqlc-dev/sqlc/cmd/sqlc@v1.30.0 generate` を呼びます。

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
| 在庫の登録 | **一度だけ**。登録済みの日を再登録すると拒否する。枠数と料金は個別の操作で変える |
| 在庫の一括登録 | **APIは持たない**。フロントエンドが日ごとに登録を投げる（複数日＝複数集約のため） |

### キャンセル料規定

**キャンセル料が発生するのは確定後（＝決済済み）の予約だけです。**
仮予約・決済処理中の予約は、規定にかかわらず 0% でキャンセルできます。
二重にキャンセルしても失敗せず、最初に確定したキャンセル料を返します（冪等）。

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
    id                *InventoryId   // roomTypeId + date
    holds             []Hold
    fee               *Fee
    quantityAvailable int
    isClosed          bool
}

type InventoryId struct {
    date       time.Time   // 日付のみ。生成時に時刻を切り捨てる
    roomTypeId uuid.UUID
}

type Hold struct {
    id        uuid.UUID
    bookingId uuid.UUID
    slotNo    int
    status    HoldStatus
    expiredAt *time.Time   // 決済中・確定済みは nil
}

type HoldStatus int
const (
    TemporaryHold HoldStatus = iota
    Confirmed
    ProcessingPayment
)
```

**`Hold` は `inventoryId` を持ちません。** 在庫集約の内部にあるため自明です。
DBの `holds` テーブルには `room_type_id` / `date` がありますが、それは外部キーとして必要なだけで、
mapper が集約の `InventoryId` から補います。

**値オブジェクトはポインタで扱います**（`*Fee`、`*InventoryId`）。
`nil` になりうる getter はポインタを返す（`ExpiredAt() *time.Time`、`FindHold() (*Hold, bool)`）。

`HoldStatus` の並びは DB の enum と一致させていません。
mapper が名前で対応させるため、`iota` の順序は保存データに影響しません。

#### メソッド

```go
func NewInventoryId(input CreateNewInventoryIdInput) (*InventoryId, error)  // Date, RoomTypeId, Now
func NewFee(input CreateNewFeeInput) (*Fee, error)
func Register(id *InventoryId, quantity int, fee *Fee) (*Inventory, error)

func (inv *Inventory) Hold(input HoldInput) (int, error)   // HoldId, BookingId, RoomTypeId, ExpiredAt, Date
func (inv *Inventory) StartPayment(bookingId uuid.UUID) error
func (inv *Inventory) Confirm(bookingId uuid.UUID) error
func (inv *Inventory) Release(bookingId uuid.UUID) error
func (inv *Inventory) CollectExpired(now time.Time) int

func (inv *Inventory) ChangeQuantity(n int) error
func (inv *Inventory) ChangeFee(f *Fee) error
func (inv *Inventory) Close() error
func (inv *Inventory) Reopen() error

func (inv *Inventory) Id() *InventoryId
func (inv *Inventory) Fee() *Fee
func (inv *Inventory) QuantityAvailable() int
func (inv *Inventory) IsClosed() bool
func (inv *Inventory) Available() int   // quantityAvailable - len(holds)
func (inv *Inventory) HoldCount() int
func (inv *Inventory) Holds() []Hold    // 複製を返す。永続化層が保存内容を読むため
func (inv *Inventory) FindHold(bookingId uuid.UUID) (*Hold, bool)

// 永続化層からの復元。検証を通さない
func Reconstruct(id *InventoryId, quantityAvailable int, fee *Fee, isClosed bool, holds []Hold) *Inventory
func ReconstructInventoryId(roomTypeId uuid.UUID, date time.Time) *InventoryId
func ReconstructHold(id, bookingId uuid.UUID, slotNo int, status HoldStatus, expiredAt *time.Time) Hold
```

#### 確保は予約IDで引く

`StartPayment` / `Confirm` / `Release` / `FindHold` は **`bookingId`** を受け取ります。
確保ID（`Hold.id`）は主キーとして存在しますが、業務上の操作は「この予約の確保を確定する」であり、
1つの在庫に同じ予約の確保が2つ存在することはありません（`Hold` が `ErrDuplicatedHold` で拒否）。
呼び出し側（予約のユースケース）が確保IDを覚えておく必要がなくなります。

確保IDは `HoldInput.HoldId` として**呼び出し側が採番**します。集約は `uuid.New()` を呼びません。

#### `HoldInput.Date` は現在時刻

名前は `Date` ですが、意味は「判定の基準となる現在時刻」です。`ExpiredAt` がこれより後であることを検証します。
`NewInventoryId` の `Now` も同様で、集約は時計を持ちません。

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
決済中（expiredAt = nil。期限切れ回収の対象外）
  ↓ Confirm
確定（expiredAt = nil。永続）
```

**「決済中」を設けているのは、決済処理中に期限切れで在庫が解放され、
課金だけが成立する事故を防ぐためです。**
`CollectExpired` は `status == TemporaryHold` かつ `expiredAt` が過去のものだけを対象とします。

### 5.2 Booking（予約）— 中核

| | |
|---|---|
| 境界 | 予約1件 |
| 実装方式 | ドメインモデル |
| 不変条件 | 状態遷移の正しさ、宿泊人数が定員以内、宿泊者が1名以上、宿泊期間の妥当性 |

```go
type Booking struct {
    id              uuid.UUID
    bookerId        uuid.UUID    // 会員集約への参照
    roomTypeId      uuid.UUID    // 施設情報集約への参照
    guests          []Guest      // 内部エンティティ
    totalFee        TotalFee
    stayPeriod      StayPeriod
    status          Status       // TemporaryHold / ProcessingPayment / Confirmed / Cancelled
    cancellationFee CancellationFee
}
```

値オブジェクト：`StayPeriod`、`TotalFee`、`Name`、`CancellationFee`。こちらは値で扱います
（在庫側がポインタなのと流儀が違うのは、実装時期の差によるもの）。

```go
func Book(id, bookerId, roomTypeId uuid.UUID, period StayPeriod,
          guests []Guest, fee TotalFee, capacity int, now time.Time) (*Booking, error)

func (b *Booking) StartPayment() error
func (b *Booking) Confirm() error
func (b *Booking) Cancel(reason CancellationReason, now time.Time) (CancellationFee, error)
func (b *Booking) FixTotalFee(f TotalFee) error   // 仮予約のときだけ
func (b *Booking) ChangeGuests(guests []Guest, capacity int) error
func (b *Booking) GuestCount() int   // len(guests)

func CalculateCancellationFee(p StayPeriod, total TotalFee, reason CancellationReason, now time.Time) CancellationFee
func Reconstruct(...) *Booking        // 永続化層からの復元
```

状態遷移は `allowedTransitions` の表で持ち、各メソッドはそれを参照します。
`Cancel` は `Cancelled` からの再呼び出しを成功として扱い、保存済みの `cancellationFee` を返します。

#### 料金は確保のあとで確定する

`Book` は `fee` を受け取りますが、実際のユースケースでは **0円で作ってから `FixTotalFee` で確定**します。
料金は日ごとの在庫が持っているため、全日程の確保が終わるまで合計が分かりません。
そして 7章の順序により、確保より先に予約行が要ります。
`FixTotalFee` は仮予約の状態でしか通らないので、決済に進んだあとに金額が動くことはありません。

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

予約集約は `holdIds` を保持しません。
確保は在庫集約に属し、DBでは `holds.booking_id` で紐づきます。
在庫側の操作が `bookingId` で引けるため（5.1）、補償や確定のときは
`StayPeriod.Dates()` の各日について予約IDで在庫を操作すれば足ります。

現在の補償は `booking/app` がメモリ上で行っています（確保できた日付を配列で持ち、失敗時に解放）。
プロセスが途中で落ちると補償は走りませんが、確保には期限があるため回収されます。
中断地点からの再開が要る段階（8章）で、プロセスマネージャーが状態をDBに持ちます。

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

論理削除（`deletedAt`）はテーブルにはありますが、**型とメソッドは未実装**です。
一覧クエリは `deleted_at IS NULL` で絞っています。

一覧の取得（`ListAccommodations`、`ListRoomTypesByAccommodationId`）と
HTTPハンドラ（`handler.go`）も同じパッケージに置きます。層を切らない方針のためです。

### 5.4 Booker（会員）— 補完

アクティブレコード。氏名と電話番号が必須、住所は `DEFAULT ''`。

**住所は現在使用しません。** 将来、予約時の宿泊者情報入力を省略する用途を想定して
フィールドとしては保持しますが、現時点では参照しません。

「予約者としての役割」も同じ `Booker` で表します（`Booking.bookerId` が参照）。

---

## 6. ダブルブッキング防止

### 多層防御

| 層 | 手段 |
|---|---|
| ドメインモデル | `len(holds) <= quantityAvailable` を検証。空き枠番号の割り当て |
| トランザクション | `SELECT ... FOR UPDATE` による行ロック。`lock_timeout` で待ちに上限 |
| **DB制約** | **`UNIQUE (date, room_type_id, slot_no)`**、`FOREIGN KEY (room_type_id, date) → inventories` |

**アプリケーション層の検証だけでは同時実行を防げません。最終防衛線をDBに置きます。**

同じ考え方を在庫の**登録**にも適用しています。二重登録を防ぐのは `inventories` の主キーで、
リポジトリは UPSERT ではなく素の INSERT を打ち、主キー違反（`23505`）を `ErrAlreadyRegistered` に翻訳します。
アプリ側で「先に読んでから判定」する方式は、同時に2本走るとすり抜けます。

### 枠番号（slot_no）方式

各確保に 1〜quantityAvailable の枠番号を割り当て、一意制約で二重取得を防ぎます。

**枠番号を決めるのはドメインモデルです。**

```go
func (inv *Inventory) vacantSlotNo() (int, bool) {
    used := make(map[int]struct{}, len(inv.holds))
    for _, h := range inv.holds {
        used[h.slotNo] = struct{}{}
    }
    for slotNo := 1; slotNo <= inv.quantityAvailable; slotNo++ {
        if _, ok := used[slotNo]; !ok { return slotNo, true }
    }
    return 0, false
}
```

`ChangeQuantity` は「確保の件数」ではなく「使用中の最大枠番号」と比較します。
解放によって枠番号が飛んでいる場合（1と3が使用中、2が空き）に、件数で判定すると
枠番号3が販売可能数を超える状態を許してしまうためです。

**DBに決めさせる案（`generate_series` を使う INSERT）は不採用です。**
理由は、空き枠を探すことが業務ロジックであり、集約が持つべきだからです。
また、DBに任せると集約のテストがDBなしで完結しません。
この方式のクエリは一度書いてから削除しました。

`SELECT FOR UPDATE` でロックを取るため実際には衝突しませんが、
`UNIQUE` 制約は最終防衛線として残します。

**解放された枠番号は再利用されます。** 枠番号に業務上の意味はなく、
実際の部屋割り当てはチェックイン時に宿が行うためです。

### デッドロック回避

複数日をまたぐ在庫のロックは、**必ず日付の昇順**で取得します。
`StayPeriod.Dates()` が昇順を保証しているため、そのままループすれば守られます。

### 検証するテスト

| テスト | 状態 | 場所 |
|---|---|---|
| 残1枠に100 goroutine が同時に確保 → 成功が正確に1件、DBの行数も1 | **通過** | `inventory/infra/postgres/repository_test.go` `TestDoubleBooking` |
| ロック待ちが上限を超えると `ErrBusy` で返る | **通過** | 同 `TestTransactor` |
| quantityAvailable を確保数より小さくしようとする → 拒否される | **通過** | `inventory/domain` |
| 3日分の在庫を複数プロセスが同時に取り合う → デッドロックが発生しない | 未着手 | — |

並行テストは `Transactor` + `Repository` + 集約だけで書きます。`Service` は使いません。
守られるべきはこの3つの組み合わせであって、`Service` の実装ではないからです。

`go test -race` を必ず使用します。

---

## 7. 結果整合性と処理順序

在庫と予約は別の集約であり、**1トランザクション1集約の原則を守ります。**

### 処理順序

`holds.booking_id` に NOT NULL の外部キーがあるため、予約を先に作ります。

```
T1: 予約を作成（status = temporary_hold、料金 0円）
T2: 12/24 の在庫を読み、確保を追加
T3: 12/25 の在庫を読み、確保を追加
T4: 12/26 の在庫を読み、確保を追加
T5: 予約の料金を確定（各日の料金の合計）
    （外部システム）決済
T6: 予約を決済処理中に
T7〜T9: 各在庫の確保を決済中に（期限を外す）
T10〜T12: 各在庫の確保を確定
T13: 予約を確定
```

**各トランザクションが1集約のみを更新しています。** `booking/app.Service` の `Book` と `Confirm` がこの順序を実装しています。

守るべき規律は2つです。

1. **確保が先、予約の確定が後。** 確定した予約の在庫が存在しない状態を防ぐ
2. **確保の確定（T7〜T12）が、予約の確定（T13）より先。** 逆にすると、確定済み予約の在庫が期限切れで消える

**在庫の確保は日付の昇順で行います**（`StayPeriod.Dates()` が保証）。全員が同じ順序でロックを取るので、
待ち合わせの輪ができません。ただし1トランザクション1集約のため、そもそも1トランザクションが持つロックは1行だけです。

途中の日が取れなかった場合は、確保できた日を解放し、予約を `Expired` 理由でキャンセルします。
このとき**0円のキャンセル済み予約が履歴に残ります**（既知の課題。12章）。

### 中間状態の許容

「在庫だけ確保されて予約が確定していない」状態は**業務的に壊れていません。**
「誰かが予約手続き中」という正常な状態であり、期限（30分）で必ず解消されます。

途中で失敗した場合は補償（解放）しますが、
**補償が失敗しても致命的ではありません。** 期限切れ回収バッチが後で拾います。二重の安全網です。

現在の補償はアプリケーション層のメモリ上で行われ、プロセスマネージャーはまだありません（8章で導入）。

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
`holdIds`（確保ID + 在庫ID）を保持し、補償時に何を解放すべきか分かるようにします。

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

#### 現在の状態

暫定のエンドポイント `POST /bookings/{id}/payment` がリクエストボディの `result` で挙動を切り替えます。

| result | 動作 |
|---|---|
| `success` | `booking/app.Confirm`（在庫の期限解除 → 確保の確定 → 予約の確定） |
| `failure` | `Cancel(PaymentFailed)`。キャンセル料 0% で在庫を解放し、402 を返す |
| `timeout` | 何もしない。予約も在庫もそのまま残し、504 を返す。期限切れ回収が決着させる |

腐敗防止層・Webhook・照合処理・プロセスマネージャーはまだありません。
決済サービスを繋いだ時点でこのエンドポイントは消えます。

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

### 現在の状態

**暫定版として、在庫テーブルを直接引いています。**`internal/search/` に置き、
`inventories` を `room_types` / `accommodations` と結合して `HAVING COUNT(*) = 泊数` で連泊を判定します。
確保数は `holds` を `LEFT JOIN` して数えます。非正規化テーブルと投影はまだありません。

読み取りモデルなので層を切らず、`ctx, db` を引数で受け取る形です。
sqlc の出力先も `internal/search/db` として独立させています。

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
    postal_code    TEXT NOT NULL,
    phone_number   TEXT NOT NULL,
    prefecture     TEXT NOT NULL,
    city           TEXT NOT NULL,
    street_address TEXT NOT NULL,
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
    'temporary_hold', 'processing_payment', 'cancelled', 'confirmed'
);

CREATE TABLE bookings (
    id            UUID PRIMARY KEY,
    booker_id     UUID NOT NULL REFERENCES bookers(id),
    room_type_id  UUID NOT NULL REFERENCES room_types(id),
    total_fee     INT  NOT NULL CHECK (total_fee >= 0),
    checkin_date  DATE NOT NULL,
    checkout_date DATE NOT NULL,
    status        booking_status NOT NULL DEFAULT 'temporary_hold',
    cancellation_fee INT NOT NULL DEFAULT 0 CHECK (cancellation_fee >= 0),  -- 000008
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT valid_period CHECK (checkout_date > checkin_date)
);

CREATE TYPE hold_status AS ENUM (
    'temporary_hold', 'processing_payment', 'confirmed'
);

CREATE TABLE holds (
    id           UUID PRIMARY KEY,
    room_type_id UUID NOT NULL REFERENCES room_types(id),
    date         DATE NOT NULL,
    booking_id   UUID NOT NULL REFERENCES bookings(id),
    slot_no      INT  NOT NULL CHECK (slot_no >= 1),
    status       hold_status NOT NULL DEFAULT 'temporary_hold',
    expired_at   TIMESTAMPTZ,             -- 決済中・確定済みは NULL
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT uq_slot UNIQUE (date, room_type_id, slot_no),
    CONSTRAINT fk_holds_inventory
        FOREIGN KEY (room_type_id, date) REFERENCES inventories (room_type_id, date)  -- 000009
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

**`holds` の複合外部キー**（000009）は、在庫を登録していない日に確保行が作れる穴を塞ぐものです。
`room_types` への単独の外部キーは冗長ですが残しています。

**`bookings.cancellation_fee`**（000008）は、二重キャンセルを冪等にするために持ちます。
キャンセル料は算出時点の日程と料金で決まり、あとから再計算できないためです。

**`holds.booking_id` が `bookings` を参照している**ため、確保より先に予約行が必要です。
これが7章の「予約を先に作る」順序を決めています。

### マイグレーション

`server/db/migrations/` に golang-migrate 形式で置きます。000001〜000007 が初期スキーマ、000008 と 000009 が上記の追加。
`make migrate`（`server/cmd/migrate`）で適用します。CLI を別途入れなくて済むよう、ライブラリを直接使う小さなコマンドです。

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
    Register(ctx context.Context, in app.RegisterInput) error
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
internal/inventory/infra/http/     在庫API（package inventoryhttp）
internal/booking/infra/http/       予約API（package bookinghttp）
internal/accommodation/handler.go  施設情報API（層を切らないのでパッケージ直下）
internal/booker/handler.go         会員API（同上）
internal/search/handler.go         空室検索API（同上）
internal/shared/http/              ミドルウェア、共通のレスポンス形式（package sharedhttp）
cmd/api/routes.go                  全ルートの一覧
cmd/api/main.go                    組み立て（合成の根）
```

**ディレクトリ名は `http` ですが、パッケージ名は `inventoryhttp` / `bookinghttp` / `sharedhttp` です。**
標準ライブラリの `net/http` と衝突させないためです。

`cmd/api/main.go` が唯一、全部を知っている場所です。
pgxpool を作り、Transactor を作り、Service を作り、ハンドラに渡し、ルータに登録します。
`cmd/api/routes.go` を見ればAPIの全体像が分かる状態を保ちます。

各文脈の `error.go` に番兵エラーとエラーコードの対応表を置きます。予約のハンドラは在庫と施設情報の
エラーも受け取るため、それらの文脈の番兵エラーも表に含めています。

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

**秒数（5秒 / 3秒）は暫定値です。** 負荷試験（13章の7）で調整します。

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
| `GET` | `/accommodations` | 200 | — |
| `POST` | `/accommodations` | 201 | `INVALID_NAME` / `INVALID_PHONE_NUMBER` / `INVALID_POSTAL_CODE` / `PREFECTURE_REQUIRED` / `CITY_REQUIRED` 400 |
| `GET` | `/accommodations/{accommodationId}` | 200 | `ACCOMMODATION_NOT_FOUND` |
| `GET` | `/accommodations/{accommodationId}/room-types` | 200 | — |
| `POST` | `/accommodations/{accommodationId}/room-types` | 201 | `INVALID_CAPACITY` / `INVALID_ROOM_TYPE_NAME` 400 / `ACCOMMODATION_NOT_FOUND` 404 |
| `GET` | `/room-types/{roomTypeId}` | 200 | `ROOM_TYPE_NOT_FOUND` |
| `POST` | `/room-types/{roomTypeId}/inventories` | 201 | `INVENTORY_ALREADY_REGISTERED` 409 / `PAST_DATE` 400 |
| `PUT` | `/room-types/{roomTypeId}/inventories/{date}/quantity` | 204 | `QUANTITY_BELOW_HOLDS` 409 / `INVALID_QUANTITY` 400 |
| `PUT` | `/room-types/{roomTypeId}/inventories/{date}/fee` | 204 | `INVALID_FEE` 400 |
| `POST` | `/room-types/{roomTypeId}/inventories/{date}/close` | 204 | `INVENTORY_ALREADY_CLOSED` 409 |
| `POST` | `/room-types/{roomTypeId}/inventories/{date}/reopen` | 204 | `INVENTORY_NOT_CLOSED` 409 |
| `GET` | `/room-types/{roomTypeId}/inventories?from=&to=` | 200 | `to` は含まない。確保数（`heldCount`）と残数（`available`）を含む |

**操作ごとにURLを分けます。** ドメインが `ChangeQuantity` / `ChangeFee` / `Close` を
別の業務操作として持っているためです。1つの `PATCH` にまとめると、
どのフィールドが来たかでハンドラが分岐することになり、判断がアダプターに漏れます。

**在庫の一括登録APIは作りません。** 複数日＝複数集約であり、
1トランザクションにまとめると7章の原則を破ります。
フロントエンドが日数ぶん並列に `POST` を投げます。
既に登録済みの日は 409 が返るので、フロント側で `PUT .../quantity` と `.../fee` に切り替えます。
一部が失敗しても残りは反映され、失敗した日数を表示します。

#### 予約者側 `/api/v1`

| メソッド | パス | 成功 | 主なエラーコード |
|---|---|---|---|
| `GET` | `/room-types/search?prefecture=&checkin=&checkout=&guests=&max_fee=` | 200 | `INVALID_STAY_PERIOD` / `INVALID_GUESTS` 400。`prefecture` と `max_fee` は省略可 |
| `GET` | `/room-types/{roomTypeId}` | 200 | `ROOM_TYPE_NOT_FOUND` 404 |
| `POST` | `/bookers` | 201 | `INVALID_FIRST_NAME` ほか 400 |
| `GET` | `/bookers/{bookerId}` | 200 | `BOOKER_NOT_FOUND` 404 |
| `GET` | `/bookers/{bookerId}/bookings` | 200 | 宿泊者は含まない要約。チェックイン日の降順 |
| `POST` | `/bookings` | 201 | `INVENTORY_SOLD_OUT` 409 / `OVER_CAPACITY` 422 / `NO_GUEST` 422 / `RESOURCE_BUSY` 503 |
| `GET` | `/bookings/{bookingId}` | 200 | `BOOKING_NOT_FOUND` 404 |
| `POST` | `/bookings/{bookingId}/payment` | 200 | `INVALID_TRANSITION` 409 / `PAYMENT_FAILED` 402 / `PAYMENT_TIMEOUT` 504 |
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
  必ず context 由来（トークンから取り出した値）に変えること。
  フロントエンド（`web/booker/src/booker.ts`）は会員IDを `localStorage` に持たせて凌いでおり、
  認証導入時に両方まとめて消す
- **補償で取り消された0円の予約が履歴に残る。** 満室で失敗した予約が
  「仮予約を作る → 確保に失敗 → キャンセル」の経路を通るため。設計どおりの動作だが利用者には見えないほうがよい
- **`GET /admin/accommodations` は全件を返す。** 認証を入れて運営者に紐づく宿だけに絞る

---

## 12. 実装の現状

### 完了

| 領域 | 状態 |
|---|---|
| イベントストーミング・設計 | 完了 |
| マイグレーション | 000001〜000009。`make migrate` で適用 |
| `accommodation` / `booker` | 型・検証・`Save`・`Find`・一覧・HTTPハンドラ |
| `inventory/domain` | 集約一式と単体テスト（46件） |
| `inventory/app` | ユースケース一式と単体テスト（41件。偽物の保存先で振る舞いを検証） |
| `inventory/infra/postgres` | Repository・Transactor（`lock_timeout` 込み）・mapper。DBテスト15件 |
| **並行テスト** | **`TestDoubleBooking` 通過**（100 goroutine、本物の PostgreSQL） |
| `booking/domain` | 集約一式と単体テスト |
| `booking/app` | 予約・確定・キャンセル・宿泊者変更・履歴。補償つき。単体テスト15件 |
| `booking/infra` | Repository・Transactor・在庫と施設情報へのアダプター |
| `search` | 空室検索の暫定版（在庫テーブルを直接引く） |
| HTTP API | 全17エンドポイント（11.7）、ミドルウェア、CORS、エラーコード写像 |
| 暫定の決済 | `POST /bookings/{id}/payment` で成功・失敗・無応答を切り替え |
| フロントエンド | `web/admin`（管理画面）と `web/booker`（予約者向け）。Claude Design のデザインを反映 |
| 起動 | `make dev` / `make seed` / `compose.yml` |

### 未着手

- **期限切れ回収ワーカー**（`cmd/worker`）。domain / app / repository は実装済みで、ループが無いだけ
- 認証（外部IdP + 腐敗防止層）
- CI・ホスティング・自動デプロイ
- プロセスマネージャー（状態をDBに持ち、中断地点から再開する）
- 決済の腐敗防止層・Webhook・照合処理
- CQRS への置き換え（非正規化テーブル、投影、再構築）
- 負荷試験
- 6章のデッドロック検証テスト（3日分を複数プロセスで取り合う）

### 既知の未完了項目

- **`Accommodation` / `RoomType` / `Booker` の ID 生成が集約内部で行われている。**
  `inventory` / `booking` は引数で受け取る形に直したが、アクティブレコード側は未修正
- `RoomType` の論理削除（型・メソッド）が未実装。テーブルの列と一覧の絞り込みだけがある
- `Booking` の状態が4つのみ（`staying` / `completed` / `no_show` は未定義）。チェックイン機能とあわせて追加する
- `guests` の年齢を持たない（用途がないため削除済み）
- 11.8 の宿題（`bookerId` の受け取り方、0円のキャンセル済み予約、宿一覧の絞り込み）

---

## 13. これからの実装順序

**動くものを早く作ることを優先します。** 設計を完璧にしてから実装するのではありません。

段階1〜6（Repository → 並行テスト → 検索 → アプリケーション層 → API → フロント）は完了しました。
以降の順序です。

```
1. 期限切れ回収ワーカー         ← 無いと在庫が固着する。作業量は最小
2. CI（並行テストを毎pushで実行）+ デッドロック検証テスト
3. 認証（外部IdP + 腐敗防止層）。bookerId をトークン由来に、/admin を運営者ロールで閉じる
4. ホスティング + 自動デプロイ
5. プロセスマネージャー + モック決済の本実装（腐敗防止層・Webhook・照合）
6. CQRS への置き換え（投影、再構築処理）
7. 負荷試験とプロファイリング
```

### 1 を最初にする理由

7章の「補償が失敗しても期限切れ回収が拾う」という安全網が、まだ存在しません。
支払わずに離脱した予約の在庫は、いま誰も解放しません。
設計が前提にしているものが無い状態なので、他の何よりも先に埋めます。

### 3・4 を 1・2 の後にする理由

公開する前提なら認証は必須ですが、期限切れ回収が無いまま公開すると在庫が固着し、
CIが無いまま公開するとテストされていない変更が本番に載ります。
認証と期限切れ回収に依存関係はないので、小さい方を先に片付けます。

### 7 の進め方

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
├── Makefile              # make dev / migrate / seed / test / sqlc
├── compose.yml           # 開発用 PostgreSQL
├── server/
│   ├── cmd/
│   │   ├── api/          # HTTPサーバー。routes.go に全ルート、main.go が合成の根
│   │   ├── migrate/      # マイグレーション適用（golang-migrate をライブラリとして使う）
│   │   ├── seed/         # 動作確認用のデモデータ
│   │   └── worker/       # 期限切れ回収、決済照合（未着手）
│   ├── internal/
│   │   ├── inventory/    # 中核・ドメインモデル
│   │   │   ├── domain/   # 集約・値オブジェクト。import は標準ライブラリと uuid だけ
│   │   │   ├── app/      # ユースケース（service.go）と、Repository / Transactor の宣言（port.go）
│   │   │   └── infra/
│   │   │       ├── postgres/  # Repository・Transactor・mapper。db/ に sqlc の生成物
│   │   │       └── http/      # 駆動アダプター（package inventoryhttp）
│   │   ├── booking/      # 中核・ドメインモデル（同構成）
│   │   │   └── infra/adapter.go   # 在庫・施設情報の文脈への窓口
│   │   ├── accommodation/  # 補完・アクティブレコード。型・永続化・ハンドラが同居
│   │   ├── booker/         # 補完・アクティブレコード
│   │   ├── search/         # 読み取りモデル。層を切らない
│   │   ├── shared/http/    # ミドルウェア、共通レスポンス（package sharedhttp）
│   │   └── testutil/       # testcontainers で PostgreSQL を立てる
│   └── db/
│       ├── migrations/
│       └── queries/      # sqlc用SQL（文脈ごとにサブディレクトリ。search も独立）
└── web/
    ├── admin/            # 管理画面（Vite + React）
    └── booker/           # 予約者向けサイト
```

**構成の不揃いは意図的です。** 中核は層を分け、補完はファイルを直に置いています。
補完領域に層を切ったり値オブジェクトを作ったりするのは過剰設計であり、
どこに手をかけるかを分類から判断した結果です。

**並行テストは `inventory/infra/postgres/repository_test.go` に置いています。**
独立した `test/concurrency/` は作りませんでした。テスト対象が Transactor と Repository なので、その隣が自然です。

**フロントエンドを `booker` と名付けた理由。** 「guest」は用語集の Guest（実際に宿泊する人）と衝突し、
権限を絞られた管理者アカウントとも読めます。サイトを使うのは予約契約の当事者＝ Booker です。

フロントエンドは通信（`api/client.ts`）と見た目（`components/ui.tsx`、`pages/`）を分けています。
デザインを差し替えるときに触るのは後者だけです。共通部品は2アプリで複製しており、共通パッケージは作りません。

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
- **現在時刻・IDは引数で受け取る。** 集約が `time.Now()` や `uuid.New()` を呼ばない。
  `inventory` / `booking` の domain 層には両方とも存在しない（grep で確認できる）
- 内部エンティティ（`Hold`）は集約が生成する。外部から直接作らせない。
  ただし識別子（`HoldId`）は呼び出し側が採番して渡す
- DBからの復元専用関数（`Reconstruct*`）は検証を通さない（保存時に検証済みのため。
  検証を厳しくしたときに既存データが読めなくなるのを防ぐ）。**テストの準備に使わない**（16章）
- 値オブジェクトの扱いは文脈で揃える。`inventory` はポインタ（`*Fee`、`*InventoryId`）、
  `booking` は値。`nil` になりうる getter はポインタを返す

### アプリケーション層（app）

- **調整はするが判断はしない。** 満室か、遷移してよいか、といった判断は domain に置く。
  `if` による業務判断がこの層に現れたら、それは domain に移すべきもの
- **「ロックして読む → 集約に操作させる → 保存する」を1トランザクションに閉じる。**
  `inventory/app.Service.mutate` がこの型で、各ユースケースは `apply` 関数を渡すだけ
- 1トランザクション1集約。複数の集約を触るユースケース（予約）は、トランザクションを集約ごとに分ける
- `Repository` / `Transactor` の宣言はこの層に置く（14章）
- 集約を組み立てない読み取り（一覧・集計）は `InventorySummary` のような表示用の型で返し、集約と区別する

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
- **識別子は `Id`（`ID` ではない）。** `InventoryId`、`bookingId`、`Id()`。
  スネークケース（`booking_id`）との対応を素直に保つため。
  `accommodation` / `booker` の `ID()` は旧規約のまま残っている

**技術用語を業務の名前に使わないこと。**
例：「在庫の一貫性」は技術用語なので「在庫管理」とする。

### コメント

**コードにコメントを書きません。** 説明はこのドキュメントに書きます。
コードとドキュメントの両方に説明があると、片方だけ直して食い違うためです。
名前で意図が伝わらないなら、コメントではなく名前を直します。

例外は、ツールが解釈する指示（`-- name:` や `/// <reference>`）と、sqlc の生成コードです。

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

| 対象 | テストの種類 | 何を確かめるか |
|---|---|---|
| 中核のドメインモデル | 単体テスト（DBなし） | 業務ルール |
| アプリケーション層 | 単体テスト（偽物の保存先） | 振る舞い。操作の結果が読み戻せるか、失敗時に状態が変わっていないか |
| アクティブレコード | 検証ロジックは DBなし、永続化は testcontainers | |
| リポジトリ・Transactor | testcontainers で本物の PostgreSQL | 集約⇄行の変換、差分保存、エラーの翻訳、ロールバック、`lock_timeout` |
| **ダブルブッキング** | **並行実行テスト（必須）** | 残1枠に100 goroutine → 成功1件 |
| HTTP | `httptest`（DBなし） | ルーティング、ミドルウェア、CORS |

### 層ごとに検証することを変える

**上の層のテストで下の層の業務ルールを繰り返しません。**
「満室なら確保できない」は domain のテストが確かめているので、app や repository のテストでは検証しません。
繰り返すと二重管理になり、「集約のテストが仕様書」という立場が崩れます。

**app 層のテストは、偽物の保存先を「失敗時にロールバックする本物」に似せます。**
読み出しは複製を返し、`WithinTx` はエラー時に状態を戻す。
これで「保存メソッドを呼んでいない」ではなく「**状態が変わっていない**」を確かめられ、
実装の呼び出し順を縛らずに済みます。例外は「状態を変えるときにロックを取る読み方をしているか」の1件だけ。

**repository のテストは `Repository` / `Transactor` / domain だけで組み立てます。**
`Service` を使うと、落ちたときにどの層が壊れたのか分からなくなります。
集約の準備には `Register` や `Hold` などの検証を通る経路を使い、`Reconstruct` は使いません。
検証を飛ばすと、本番では存在しえない状態を作れてしまうためです。
「既に期限切れの確保」は `Hold` に過去の `Date` と `ExpiredAt` を渡せば正規の経路で作れます。

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
make test         # 全テスト（DBを含む。Docker が要る）+ フロントのビルド検査
make test-short   # DB不要な高速テストのみ
```

DBを使うテストは `testing.Short()` で読み飛ばします。
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