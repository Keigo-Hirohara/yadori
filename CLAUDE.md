# yadori 設計ドキュメント

宿泊予約アプリ「yadori」の設計判断とその根拠をまとめたものです。
ドメイン駆動設計（Vlad Khononov『ドメイン駆動設計をはじめよう』オライリー・ジャパン、2024年）に基づいて設計しています。

このドキュメントは**設計判断の記録**です。コードを書く際は、ここに書かれた方針から逸脱しないでください。
方針を変更する必要が生じた場合は、変更前に理由をこのドキュメントに追記してください。

---

## 1. プロジェクトの目的

宿泊施設の空室検索・予約を行うWebアプリケーション。

**技術的な主題は、ダブルブッキングを起こさない在庫管理の設計と実装です。**
並行制御、失敗経路の洗い出し、それらを検証するテストが本プロジェクトの中心です。

### 技術スタック

| 領域 | 選定 | 理由 |
|---|---|---|
| 言語 | Go | |
| DB | PostgreSQL | 行ロック、CHECK制約、部分インデックスが必要 |
| DB層 | sqlc + pgx | 集約とテーブルを分離できる。SQLを制御できる |
| HTTP | 標準 `net/http` | Go 1.22以降、ルーティングに十分な機能がある |
| マイグレーション | golang-migrate | |
| テスト | 標準 `testing` + `testify/require` | |
| DBテスト | testcontainers-go | 並行テストに実DBが必要 |
| フロントエンド | 別途 | バックエンドに専念する |

**ORM（GORM等）は使いません。** ドメインモデルを採用しており、構造体タグでテーブルと対応させる方式は
集約が永続化を知らないという設計方針と衝突するためです。

### スコープ

| 含める | 除外する |
|---|---|
| 客側：検索・予約・変更・キャンセル・チェックイン | 宿の新規登録フロー |
| 宿側：在庫（販売枠）設定・料金設定・クローズアウト・宿都合キャンセル | 売上レポート、分析 |
| | 権限管理、複数スタッフ |
| | レビュー機能 |

決済（Stripe）と認証は外部サービスを利用します。

---

## 2. 業務領域の分類

『ドメイン駆動設計をはじめよう』1章に基づく分類です。

| 業務領域 | 複雑さ | 差別化 | 分類 | 実装方式 |
|---|---|---|---|---|
| 在庫管理 | 複雑 | しない | **一般**（※） | ドメインモデル |
| 予約管理 | 複雑 | する | **中核** | ドメインモデル |
| 施設情報の管理 | 単純 | しない | **補完** | アクティブレコード |
| 会員管理 | 単純 | しない | **補完** | アクティブレコード |
| 空室検索 | — | — | 在庫管理の読み取り側 | 読み取りモデル（CQRS） |
| 決済 | 複雑 | しない | **一般** | 外部サービス＋腐敗防止層 |
| 認証 | 複雑 | しない | **一般** | 外部サービス＋腐敗防止層 |

### ※ 在庫管理の扱いについて

**事業としては「一般の業務領域」です。** ダブルブッキングを防ぐことは業界標準であり、
どのOTAも実現しているため差別化にはなりません。実際の事業なら既製品の採用を検討すべき領域です。

**しかし本プロジェクトでは、意図的に中核として自作します。**
本プロジェクトの目的は技術力の証明であり、並行制御の設計と実装がその中心となるためです。

この2つの判断は別の理由に基づくものであり、混同してはいけません。

---

## 3. 区切られた文脈

4つの文脈に分割しています。

```
【文脈】施設情報の管理   → Accommodation 集約、RoomType 集約
【文脈】販売枠の管理     → Inventory 集約
【文脈】予約管理         → Booking 集約
【文脈】会員管理         → Booker 集約
```

### 境界を引いた根拠

**言葉の意味が変わる場所で切っています。**

| 言葉 | 施設情報の管理 | 販売枠の管理 | 予約管理 |
|---|---|---|---|
| 部屋タイプ | 商品の定義。定員・設備 | 日付と結びついた販売枠 | 予約対象の種別 |
| 在庫 | （存在しない） | 売りに出す枠の数 | （存在しない。確保IDのみ知る） |
| 部屋 | 物理的な部屋 | （扱わない） | チェックイン時に割り当てられるもの |

加えて、**変更理由が独立していること**も根拠です。
施設情報は年に数回しか変わらず、在庫は毎日変わります。

### 業務領域と文脈は一致しません

本書の立場に従い、業務領域（事業活動の分野）と区切られた文脈（モデルの適用範囲）は
別の基準で線を引いています。今回は結果としてほぼ1対1になっていますが、
**実装方式は業務領域ごと（集約単位）に決まる**という原則を保ちます。

---

## 4. 業務ルール

実装前に確定させた業務上の判断です。

| 項目 | 決定 |
|---|---|
| 支払い方式 | **前払い**（決済完了をもって予約確定） |
| 仮確保の期限 | **30分** |
| 決済中の上限 | **30分**。超過分は照合処理が拾って決着させる |
| 販売可能数の変更 | **確保済み数を下回る変更は拒否する** |
| クローズアウト | `IsClosed` フラグで表現。**枠数は保持したまま新規確保のみ拒否** |
| 部屋タイプの削除 | **論理削除**。既存予約は有効、新規の在庫公開は不可、検索には出さない |
| 宿泊者情報 | **予約時に全員分を必須入力**。後日入力は不可 |
| 宿泊人数 | `Guests` の件数から導出する。独立したフィールドを持たない |

### キャンセル料規定

| タイミング | 料率 |
|---|---|
| 7日前まで | 0% |
| 3〜6日前 | 50% |
| 前日・当日 | 100% |
| 宿都合 | **0%**（理由を問わず） |

宿都合のキャンセルは客都合と業務ルールが根本的に異なるため、
`CancellationReason` によって算出を分岐させます。

### クローズアウトを `IsClosed` にした理由

`QuantityAvailable = 0` で表現する案も検討しましたが、以下の理由で採用しませんでした。

- 元の枠数が失われ、販売再開時に復元できない
- 「元々出していない」「売り切れた」「宿が止めた」が区別できない
- 確保済みの枠がある日は、不変条件により 0 に変更できない
  （既存予約を維持したまま新規受付だけ止める、という運用ができなくなる）

---

## 5. 集約の設計

型定義は設計上の構造を示すものです。Go の記法で書いていますが、
フィールドはすべて非公開、参照はゲッター経由とします。

### 5.1 Accommodation（宿）

| | |
|---|---|
| 境界 | 1宿。RoomType は別集約として分離 |
| 実装方式 | アクティブレコード |
| 不変条件 | 各フィールドの形式（薄い） |

```go
type Accommodation struct {
    id            uuid.UUID
    name          string
    phoneNumber   string   // ハイフン除去済み
    postalCode    string   // ハイフン除去済み
    prefecture    string
    city          string
    streetAddress string
    building      string   // 空を許容
}

type AccommodationCreateInput struct {   // フィールドは公開
    Name          string
    PhoneNumber   string
    PostalCode    string
    Prefecture    string
    City          string
    StreetAddress string
    Building      string
}

func New(in AccommodationCreateInput) (Accommodation, error)
func (a Accommodation) ID() uuid.UUID
func (a Accommodation) Name() string
// 以下、各フィールドのゲッター
```

#### 検証ルール

| フィールド | ルール |
|---|---|
| Name | trim 後 1〜60文字（`utf8.RuneCountInString` で数える） |
| PhoneNumber | ハイフン除去後 `^0\d{9,10}$` |
| PostalCode | ハイフン除去後 `^\d{7}$` |
| Prefecture | 空でない |
| City | 空でない |
| StreetAddress | 空でない |
| Building | 空を許容 |

#### 正規化

`New` の中で、Name の前後空白除去、PhoneNumber と PostalCode のハイフン除去を行います。

### 5.2 RoomType（部屋タイプ）

| | |
|---|---|
| 境界 | 部屋タイプ1件 |
| 実装方式 | アクティブレコード |
| 不変条件 | **定員が1以上**（予約集約の不変条件で使用するため厳格に検証する） |

```go
type RoomType struct {
    id              uuid.UUID
    accommodationID uuid.UUID
    name            string
    capacity        int
    hasPrivateBath  bool
    hasBalcony      bool
    deletedAt       *time.Time   // 論理削除
}

func New(in RoomTypeCreateInput) (RoomType, error)
func (r RoomType) IsDeleted() bool
func (r RoomType) Delete() RoomType    // deletedAt を設定した新インスタンスを返す
```

補完領域だが、**他の集約が依存する値（定員）の検証は省略しないこと。**

### 5.3 Inventory（在庫）

| | |
|---|---|
| 境界 | **部屋タイプ × 日付** の1インスタンス |
| 実装方式 | ドメインモデル |
| 技術方式 | ポートとアダプター + CQRS |
| 不変条件 | **確保済み数 ≤ 販売可能数**（＝ダブルブッキングを起こさない） |

```go
// 集約ルート
type Inventory struct {
    id                InventoryID
    holds             []Hold
    fee               Fee
    quantityAvailable int
    isClosed          bool
}

// 値オブジェクト（識別子）
type InventoryID struct {
    roomTypeID uuid.UUID
    date       time.Time   // 日付のみ。時刻は持たない
}

// 内部エンティティ
type Hold struct {
    id       uuid.UUID
    slotNo   int          // 1〜quantityAvailable
    status   HoldStatus
    expiresAt *time.Time  // 確定済みは nil
}

type HoldStatus int
const (
    TemporaryHold HoldStatus = iota   // 仮確保。期限あり
    ProcessingPayment                 // 決済中。期限切れ回収の対象外
    Confirmed                         // 確定。永続
)

// 値オブジェクト
type Fee struct {
    amount int   // 円。浮動小数点を使わない
}
```

#### メソッド

```go
// 生成
func Publish(id InventoryID, quantity int, fee Fee) (*Inventory, error)

// 宿側の操作
func (inv *Inventory) ChangeQuantity(n int) error   // 確保数を下回る変更は拒否
func (inv *Inventory) ChangeFee(f Fee) error
func (inv *Inventory) Close() error                 // クローズアウト
func (inv *Inventory) Reopen() error

// 確保のライフサイクル
func (inv *Inventory) Hold(holdID uuid.UUID, expiresAt time.Time) (int, error)  // slotNo を返す
func (inv *Inventory) StartPayment(holdID uuid.UUID) error   // 期限を停止
func (inv *Inventory) Confirm(holdID uuid.UUID) error        // 期限を外す
func (inv *Inventory) Release(holdID uuid.UUID) error
func (inv *Inventory) CollectExpired(now time.Time) int      // 期限切れを回収し件数を返す

// 参照
func (inv *Inventory) ID() InventoryID
func (inv *Inventory) Available() int    // quantityAvailable - len(holds)
func (inv *Inventory) Fee() Fee
func (inv *Inventory) IsClosed() bool
```

#### この粒度にした理由

**本プロジェクトの中核的な設計判断です。**

- 宿単位にすると、12/24を予約する人と3/10を予約する人が同じロックを奪い合う
- 全期間を1集約にすると、予約が日付をまたぐため芋づる式に肥大化する
- 部屋タイプ × 日付なら、3泊の予約でも3インスタンスで済み、無関係な日付と競合しない

**集約の境界はロックの粒度でもある**、という理解に基づく判断です。

#### 在庫は部屋タイプの実体を持ちません

`roomTypeID` のみを参照します。在庫が守る不変条件は「数」だけであり、
部屋タイプの名前・設備・定員は判定に不要なためです。
**集約をまたぐ参照はIDで行う**という原則に従います。

定員チェックは予約集約の責務です。施設情報から定員を取得し、値として渡します。

#### Hold の状態遷移

```
仮確保（期限あり）
  ↓ StartPayment
決済中（期限を停止。回収対象外）
  ↓ Confirm
確定（期限なし。永続）
```

**「決済中」を設けているのは、決済処理中に期限切れで在庫が解放され、
課金だけが成立する事故を防ぐためです。**

`CollectExpired` は `status == TemporaryHold` かつ `expiresAt` が過去のものだけを対象とします。

### 5.4 Booking（予約）

| | |
|---|---|
| 境界 | 予約1件 |
| 実装方式 | ドメインモデル |
| 技術方式 | ポートとアダプター |
| 不変条件 | 状態遷移の正しさ、宿泊人数が定員以内、宿泊期間の妥当性、宿泊者が1名以上 |

```go
// 集約ルート
type Booking struct {
    id         uuid.UUID
    bookerID   uuid.UUID    // 予約者。会員集約への参照
    roomTypeID uuid.UUID    // 施設情報集約への参照
    guests     []Guest      // 内部エンティティ
    holdIDs    []uuid.UUID  // 在庫集約への参照（実体は持たない）
    totalFee   TotalFee
    stayPeriod StayPeriod
    status     Status
}

// 内部エンティティ
type Guest struct {
    id   uuid.UUID
    name Name
}

// 値オブジェクト
type Name struct {
    firstName string
    lastName  string
}

type StayPeriod struct {
    checkinDate  time.Time
    checkoutDate time.Time
}

type TotalFee struct {
    amount int
}

type Status int
const (
    TemporaryHold Status = iota   // 仮予約
    ProcessingPayment             // 決済処理中
    Confirmed                     // 確定
    Cancelled
    Staying
    Completed
    NoShow
)
```

#### メソッド

```go
// 生成。在庫確保後に呼ばれる
func Book(in BookingCreateInput, capacity int) (*Booking, error)

// 状態遷移
func (b *Booking) StartPayment() error
func (b *Booking) Confirm() error
func (b *Booking) Cancel(reason CancellationReason, now time.Time) (CancellationFee, error)
func (b *Booking) CheckIn(now time.Time) error
func (b *Booking) CheckOut() error
func (b *Booking) MarkNoShow() error

// 変更
func (b *Booking) ChangeGuests(guests []Guest, capacity int) error
func (b *Booking) ChangeStayPeriod(p StayPeriod, newHoldIDs []uuid.UUID, newFee TotalFee) error
func (b *Booking) ChangeRoomType(roomTypeID uuid.UUID, newHoldIDs []uuid.UUID, newFee TotalFee, capacity int) error

// 参照
func (b *Booking) GuestCount() int    // len(guests)。人数はここから導出する
```

#### 値オブジェクトの振る舞い

```go
func NewStayPeriod(checkin, checkout time.Time) (StayPeriod, error)  // checkout > checkin を検証
func (p StayPeriod) Nights() int
func (p StayPeriod) Dates() []time.Time   // 在庫を確保する日付一覧。3泊なら3件
func (p StayPeriod) Overlaps(other StayPeriod) bool
```

`Dates()` が在庫確保の対象日を導出します。プロセスマネージャーがこれを使います。

#### 状態遷移

```
仮予約 → 決済処理中 → 確定 → 滞在中 → 完了
  ↓         ↓          ↓
キャンセル済            不泊
```

遷移可能な組み合わせは遷移表として一箇所にまとめ、各メソッドがそれを参照します。

#### 定員チェックは値を受け取って行います

`Book` と `ChangeGuests` は `capacity int` を引数で受け取ります。
予約集約が施設情報集約に依存しないようにするためです。
呼び出し側（アプリケーション層）が RoomType から取得して渡します。

#### 確定料金は予約時点でコピーします

料金プランを参照し続けてはいけません。宿が後から値上げした場合に既存予約の金額が変わるのは、
業務上も法的にも許されないためです。値オブジェクトとして予約集約内に固定します。

#### キャンセル料の算出

```go
type CancellationReason int
const (
    ByGuest CancellationReason = iota      // 客都合
    ByAccommodation                        // 宿都合。料金0
    PaymentFailed
    Expired
)

func CalculateCancellationFee(p StayPeriod, total TotalFee, reason CancellationReason, now time.Time) CancellationFee
```

将来、宿ごとにキャンセル規定が変わる可能性があるため、独立した関数として切り出しておきます。

### 5.5 Booker（会員）

| | |
|---|---|
| 境界 | 会員1件 |
| 実装方式 | アクティブレコード |
| 不変条件 | （薄い） |

```go
type Booker struct {
    id            uuid.UUID
    firstName     string
    lastName      string
    postalCode    string   // string。int にすると先頭の0が消える
    phoneNumber   string   // 同上
    prefecture    string
    city          string
    streetAddress string
    building      string
}
```

**住所は今回の機能では使用しません。** 将来、予約時の宿泊者情報入力を省略する用途を想定して
フィールドとしては保持しますが、現時点では参照しません。

「予約者としての役割」も同じ `Booker` で表します。`Booking.bookerID` がこれを参照します。

---

## 6. 在庫と予約を分けた理由と、その帰結

### 分けた理由

在庫と予約は強い整合性を要求しますが、**同じ集約にすると肥大化します。**

3泊の予約は3日分の在庫に関係します。予約を軸にまとめると、隣接する予約が次々に連鎖し、
最終的にその部屋タイプの全期間が1集約になります。すると無関係な日付の予約が直列化します。

### 分けた帰結：整合性の担保方法

分けた以上、1トランザクションで両方を更新できません（1トランザクション1集約の原則）。

**本プロジェクトでは原則を守り、時限付き確保とプロセスマネージャーで結果整合性を実現します。**

```
① 12/24の在庫を確保        （トランザクション1・1集約）
② 12/25の在庫を確保        （トランザクション2・1集約）
③ 12/26の在庫を確保        （トランザクション3・1集約）
④ 予約集約を作成           （トランザクション4・1集約）
⑤ 決済を依頼               （外部システム）
⑥ 確保を確定・予約を確定
```

途中で失敗した場合は、確保済みのものを解放します（補償）。
プロセスが落ちた場合も、確保には期限があるため必ず回収されます。

**「在庫だけ確保されて予約がない」状態は業務的に壊れていません。**
「誰かが予約手続き中である」という正常な状態を表しており、期限で必ず解消されるためです。

#### 順序の規律

- **必ず在庫を先に確保し、予約の確定は最後**。逆順にすると確定済み予約の在庫が存在しない状態が生じます
- 予約確定時は、**確保の期限を外す処理を先に行う**
- 複数日をまたぐ在庫のロックは、**必ず日付の昇順**で取得（デッドロック回避）

### 検討したが採用しなかった案

**在庫3件＋予約1件を1トランザクションで更新する案**（1トランザクション1集約からの逸脱）。

同一DBに収まっており技術的には可能で、実装も単純です。
実務ではこちらが妥当な場面も多いと考えられます。

採用しなかったのは、本プロジェクトが原則に忠実な設計の実践を目的としているためです。
また、この方式では9章（サーガ、プロセスマネージャー、アウトボックス）を実践する機会がなくなります。

---

## 7. ダブルブッキング防止の実装方針

### 枠番号（slot_no）方式を採用します

各確保に **1〜quantity_available の枠番号**を割り当て、
`UNIQUE (room_type_id, date, slot_no)` で二重取得を防ぎます。

```
quantity_available = 3 の日
  slot_no=1 → hold_A
  slot_no=2 → hold_B
  slot_no=3 → hold_C
  → 4人目は取れる番号がなく、確保に失敗する
```

**カウンタ（`reserved` 列）を持たないため、集約の状態とDBがズレようがありません。**
確保数は `inventory_holds` の行数そのものです。

カウンタ方式（`CHECK (reserved <= capacity)`）も検討しましたが、
`reserved` と行数の一致をアプリケーション側で保証し続ける必要があるため採用しませんでした。

### 空き枠の取得

「使われていない最小の番号」を1文のINSERTで取得します。

```sql
INSERT INTO inventory_holds (hold_id, room_type_id, date, slot_no, status, expires_at)
SELECT $1, $2, $3, s.slot_no, 0, $4
FROM generate_series(
    1,
    (SELECT quantity_available FROM inventories WHERE room_type_id = $2 AND date = $3)
) AS s(slot_no)
WHERE NOT EXISTS (
    SELECT 1 FROM inventory_holds h
    WHERE h.room_type_id = $2 AND h.date = $3 AND h.slot_no = s.slot_no
)
ORDER BY s.slot_no
LIMIT 1
RETURNING slot_no;
```

**挿入行数が0なら満室**です。同時実行で同じ番号を狙っても、一意制約により片方が失敗します。

### 多層防御

| 層 | 手段 |
|---|---|
| ドメインモデル | 在庫集約が `len(holds) ≤ quantityAvailable` を検証 |
| トランザクション | `SELECT ... FOR UPDATE` による行ロック |
| **DB制約** | **`UNIQUE (room_type_id, date, slot_no)`** |

**アプリケーション層の検証だけでは同時実行を防げません。最終防衛線をDBに置きます。**

### デッドロック回避

複数日をまたぐ在庫のロックは、**必ず日付の昇順**で取得します。
リポジトリの `FindForUpdate` が受け取った ID をソートしてからロックを取ります。

### 検証するテスト

```
・残1枠に100 goroutine が同時に確保を試みる
  → 成功が正確に1件、失敗が99件
  → DBの inventory_holds の行数が1であること
・3日分の在庫を複数プロセスが同時に取り合う
  → デッドロックが発生しないこと
・quantity_available を確保数より小さい値に変更しようとする
  → 拒否されること
```

`go test -race` を必ず使用します。

---

## 8. 決済連携（外部システム）

決済は外部サービスのため、DBトランザクションが届きません。
**「片方だけ完了している」状態が原理的に避けられないため、復旧手段を用意します。**

### 対策

**① プロセスマネージャーが状態をDBに保持する**

各ステップの完了後に必ず保存します。どこまで進んだかが記録されるため、中断地点から再開できます。

**② 同期レスポンスとWebhookの二重経路**

片方を取りこぼしても、もう片方で確定処理が実行されます。
両方届く可能性があるため、**受信処理は冪等**に実装します（決済IDで処理済みを判定）。

**③ 定期的な照合**

「決済依頼済みだが確定していない」予約を検出し、決済サービスに実際の課金状況を問い合わせます。
課金済みなら予約を確定（救済）、未課金なら在庫を解放します。

### 腐敗防止層

決済サービスのレスポンスをそのまま業務ロジックに渡しません。
自分たちの言葉（`決済結果`）に変換する層を挟み、外部の仕様変更が業務ロジックに波及しないようにします。

---

## 9. 空室検索（CQRS）

### 分ける理由

在庫集約は「部屋タイプ × 日付」の粒度であり、
「千葉県で12/24から2泊、大人2名、15,000円以下」という検索に答えられません。

**書き込み側（在庫集約）と読み取り側（検索モデル）で要求が正反対**のため、モデルを分離します。

**CQRSはイベント履歴式（イベントソーシング）とは独立した技術方式です。**
本プロジェクトはイベント履歴式を採用しませんが、CQRSは採用します。

### 読み取りモデル

検索用テーブルは非正規化します。宿名・住所・定員などを結合なしで引ける形にします。

連泊の判定は `HAVING COUNT(*) = 泊数` で行います（全日程に空きがある部屋タイプだけが残る）。

### 投影

**同期投影**とし、在庫の更新と同じトランザクションで検索テーブルを更新します。

投影の呼び出しは**リポジトリの `Save` の中の一箇所のみ**とします。
各コマンドに投影処理を書くと、コマンドが増えたときに書き忘れが発生するためです。

検索テーブルは在庫集約から再構築可能な派生データです。
不整合が生じた場合に備え、**全削除して作り直す処理を用意します。**

---

## 10. DBスキーマ

**集約とテーブルは1対1ではありません。** 集約は読み書きの単位、テーブルは保存の形です。
値オブジェクトは親テーブルのカラムに展開し、別テーブルにしません。

### 施設情報

```sql
CREATE TABLE accommodations (
    id             UUID PRIMARY KEY,
    name           TEXT NOT NULL,
    phone_number   TEXT NOT NULL,   -- ハイフン除去済み
    postal_code    TEXT NOT NULL,   -- ハイフン除去済み
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
    deleted_at       TIMESTAMPTZ,      -- 論理削除
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 有効な部屋タイプの検索用
CREATE INDEX idx_room_types_active ON room_types (accommodation_id)
    WHERE deleted_at IS NULL;
```

### 在庫

```sql
CREATE TABLE inventories (
    room_type_id       UUID NOT NULL REFERENCES room_types(id),
    date               DATE NOT NULL,          -- 時刻は持たない
    quantity_available INT  NOT NULL CHECK (quantity_available >= 0),
    fee_amount         INT  NOT NULL CHECK (fee_amount >= 0),  -- 円。浮動小数点は使わない
    is_closed          BOOLEAN NOT NULL DEFAULT false,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (room_type_id, date)   -- 集約の識別子をそのまま主キーにする
);

CREATE TABLE inventory_holds (
    hold_id      UUID PRIMARY KEY,
    room_type_id UUID NOT NULL,
    date         DATE NOT NULL,
    slot_no      INT  NOT NULL CHECK (slot_no >= 1),
    status       SMALLINT NOT NULL,     -- 0:仮確保 1:決済中 2:確定
    expires_at   TIMESTAMPTZ,           -- 確定済みは NULL
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    FOREIGN KEY (room_type_id, date) REFERENCES inventories (room_type_id, date),

    -- ダブルブッキング防止の最終防衛線
    CONSTRAINT uq_slot UNIQUE (room_type_id, date, slot_no)
);

-- 期限切れ回収用。仮確保の行だけを対象にする部分インデックス
CREATE INDEX idx_holds_expiring ON inventory_holds (expires_at)
    WHERE status = 0;
```

**主キーを自然キー `(room_type_id, date)` にしている**のは、
集約の識別子とテーブルの主キーを一致させ、モデルとの対応を読み取りやすくするためです。

### 予約

```sql
CREATE TABLE bookings (
    id            UUID PRIMARY KEY,
    booker_id     UUID NOT NULL REFERENCES bookers(id),
    room_type_id  UUID NOT NULL REFERENCES room_types(id),
    checkin_date  DATE NOT NULL,
    checkout_date DATE NOT NULL,
    total_fee     INT  NOT NULL CHECK (total_fee >= 0),
    status        SMALLINT NOT NULL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- StayPeriod の不変条件を DB でも守る
    CONSTRAINT valid_period CHECK (checkout_date > checkin_date)
);

CREATE TABLE booking_guests (
    id         UUID PRIMARY KEY,
    booking_id UUID NOT NULL REFERENCES bookings(id) ON DELETE CASCADE,
    first_name TEXT NOT NULL,
    last_name  TEXT NOT NULL
);

-- 予約が確保している在庫への参照
CREATE TABLE booking_holds (
    booking_id UUID NOT NULL REFERENCES bookings(id) ON DELETE CASCADE,
    hold_id    UUID NOT NULL,   -- 外部キーは張らない（後述）
    PRIMARY KEY (booking_id, hold_id)
);
```

`booking_guests` の `ON DELETE CASCADE` は、**宿泊者が予約集約の内部である**ことの表現です。
予約なしに宿泊者は存在しません。

`booking_holds.hold_id` に**外部キーを張らない**のは、確保と予約のライフサイクルが独立しているためです。
期限切れで確保が削除されるとき、外部キーがあると削除できなくなります。

### 会員

```sql
CREATE TABLE bookers (
    id             UUID PRIMARY KEY,
    first_name     TEXT NOT NULL,
    last_name      TEXT NOT NULL,
    postal_code    TEXT NOT NULL,   -- string。int にすると先頭の0が消える
    phone_number   TEXT NOT NULL,
    prefecture     TEXT NOT NULL,
    city           TEXT NOT NULL,
    street_address TEXT NOT NULL,
    building       TEXT NOT NULL DEFAULT '',
    created_at     TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);
```

### プロセスマネージャー

```sql
CREATE TABLE booking_processes (
    id           UUID PRIMARY KEY,
    booking_id   UUID NOT NULL,
    status       SMALLINT NOT NULL,   -- 0:在庫確保中 1:決済待ち 2:完了 3:失敗
    hold_ids     JSONB NOT NULL DEFAULT '[]',   -- 補償に必要
    payment_id   TEXT,
    error_reason TEXT,
    started_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- 止まっているプロセスを検出する
CREATE INDEX idx_processes_stuck ON booking_processes (status, updated_at)
    WHERE status IN (0, 1);
```

`hold_ids` を JSONB にしているのは、補償時に解放すべき確保を記録するためです。
プロセスと一体で読み書きするので、別テーブルにしません。

### 検索用テーブル（読み取りモデル）

**部屋タイプ単位の行**とします。1宿に複数行できるため、
宿単位でまとめるには `GROUP BY accommodation_id` が必要です。

```sql
CREATE TABLE room_availability (
    room_type_id       UUID NOT NULL,
    date               DATE NOT NULL,

    -- 在庫から
    available_count    INT  NOT NULL,
    fee_amount         INT  NOT NULL,
    is_closed          BOOLEAN NOT NULL,

    -- 施設情報から非正規化（結合なしで検索するため）
    accommodation_id   UUID NOT NULL,
    accommodation_name TEXT NOT NULL,
    prefecture         TEXT NOT NULL,
    city               TEXT NOT NULL,
    room_type_name     TEXT NOT NULL,
    capacity           INT  NOT NULL,
    has_private_bath   BOOLEAN NOT NULL,
    has_balcony        BOOLEAN NOT NULL,

    PRIMARY KEY (room_type_id, date)
);

CREATE INDEX idx_availability_search
    ON room_availability (prefecture, date, fee_amount)
    WHERE available_count > 0 AND is_closed = false;
```

**正規化の原則にあえて反しています。** 読み取り専用モデルは非正規化するのが正しい形です。

### 連泊検索のクエリ

```sql
SELECT accommodation_id, accommodation_name, room_type_id, SUM(fee_amount) AS total_fee
FROM room_availability
WHERE prefecture = $1
  AND date >= $2 AND date < $3
  AND available_count > 0
  AND is_closed = false
  AND capacity >= $4
GROUP BY room_type_id, accommodation_id, accommodation_name
HAVING COUNT(*) = $5           -- 泊数。全日程に空きがある部屋タイプだけが残る
   AND SUM(fee_amount) <= $6;
```

---

## 11. 用語集

業務の言葉で設計し、コードにもその言葉が現れるようにします。

| 用語 | 定義 |
|---|---|
| 在庫 | 特定の日・特定の部屋タイプについて、販売可能な枠の数 |
| 確保 | 在庫を一時的に押さえること。期限あり |
| 確定 | 決済完了後、確保が永続化された状態 |
| クローズアウト | 宿の判断で、特定の日の販売を停止すること |
| 枠番号（slot_no） | 在庫1枠を識別する 1〜quantity_available の番号 |
| Booker（会員／予約者） | サイトに登録した利用者アカウント。予約契約の当事者でもある |
| Host（宿管理人） | 宿を運営する側のアカウント |
| Guest（宿泊者） | 実際に宿泊する人。予約者と異なる場合がある |

### 命名の原則

**その言葉を業務エキスパートが使うかどうかで判断します。**

- 業務領域・文脈・集約の名前 → 業務の言葉
- 集約のメソッド名 → 業務の言葉（`Hold`、`CheckIn`）
- リポジトリ、投影、プロセスマネージャー → 技術の言葉でよい（業務に対応物がないため）

技術用語を業務の名前に使わないこと。
例：「在庫の一貫性」は技術用語なので「在庫管理」とする。

### 実装上の命名規則

- 型名・関数名・フィールド名：英語
- エラーメッセージ：日本語
- ファイル名：スネークケース（`room_type.go`）

---

## 12. ディレクトリ構成

**業務領域（文脈）でトップレベルを切り、その内側を実装方式に応じて構成します。**

技術レイヤー（`models/`、`handlers/`）でトップレベルを切りません。
変更の単位は業務領域であり、層ではないためです。

```
yadori/
├── cmd/
│   ├── api/main.go              # HTTPサーバー
│   └── worker/main.go           # 期限切れ回収、決済照合
│
├── internal/
│   ├── inventory/               # 販売枠の管理（ドメインモデル）
│   │   ├── domain/              # 業務ロジック層
│   │   │   ├── inventory.go
│   │   │   ├── hold.go
│   │   │   ├── inventory_id.go
│   │   │   ├── repository.go    # インターフェース定義はここ
│   │   │   └── errors.go
│   │   ├── app/                 # アプリケーション層
│   │   └── infra/               # インフラストラクチャ層
│   │       ├── postgres/
│   │       └── http/
│   │
│   ├── booking/                 # 予約管理（ドメインモデル）
│   │   ├── domain/
│   │   ├── app/
│   │   │   └── process.go       # プロセスマネージャー
│   │   └── infra/
│   │
│   ├── accommodation/           # 施設情報（アクティブレコード）
│   │   ├── accommodation.go     # レイヤーを切らない
│   │   ├── room_type.go
│   │   └── http/
│   │
│   ├── booker/                  # 会員（アクティブレコード）
│   │
│   ├── search/                  # 空室検索（読み取りモデル）
│   │   ├── query.go
│   │   └── projection.go
│   │
│   └── shared/
│       ├── db/
│       └── payment/             # 決済の腐敗防止層
│
├── db/
│   ├── migrations/
│   └── queries/                 # sqlc用SQL
└── test/
    └── concurrency/             # 並行テスト
```

### 構成の不揃いは意図的です

```
inventory/     → domain / app / infra の3層
accommodation/ → ファイルが直に置かれている
```

**この違いが「ここは中核、ここは補完」という設計判断の表明です。**
補完領域に層を切ったり値オブジェクトを作ったりするのは過剰設計であり、本書が否定している行為です。

### 依存の向き

```
infra → app → domain
```

インターフェースは利用側（`domain`）で定義し、実装を `infra` に置きます。
**`domain` は `infra` を import しません。** Goの循環import禁止により、これは言語レベルで保証されます。

---

## 13. 実装方式ごとの規約

### ドメインモデル（inventory, booking）

- **フィールドは非公開**。集約ルートのメソッド経由でのみ操作する
- 集約は**永続化を知らない**。SQLもORMもドメイン層に登場させない
- 集約をまたぐ参照は**IDで行う**。他集約の実体を保持しない
- 値オブジェクトは**イミュータブル、IDを持たない**
- 状態を変える操作は、単純に見えても**必ず集約のメソッド経由**とする
  （参照のみ集約を経由しなくてよい）

### アクティブレコード（accommodation, booker）

- **値オブジェクトを作らない**。住所はフラットなフィールドとして持つ
- **リポジトリインターフェースを作らない**
- **レイヤー分けをしない**
- **ドメインイベントを発行しない**
- フィールドは非公開とし、参照はゲッター経由
- **更新は新しいインスタンスを作り直す**（セッターを作らない）
- 検証はファクトリ関数 `New` の中で行う
- **入力用の構造体のフィールドは公開する**（パッケージ外から呼べる必要があるため）

補完領域だが、**他の集約が依存する値（RoomType の定員）の検証は省略しない。**

### エラー

番兵エラー（`var ErrXxx = errors.New(...)`）を定義し、`errors.Is` で判定可能にします。
`fmt.Errorf` で都度生成すると、呼び出し側が種類を判定できません。

---

## 14. テスト方針

| 対象 | テストの種類 |
|---|---|
| 中核のドメインモデル | 単体テスト（DBなし）。業務ルールを検証 |
| アクティブレコード | 検証ロジックの単体テスト + DB込みの統合テスト |
| 集約をまたぐ整合性 | 統合テスト |
| **ダブルブッキング** | **並行実行テスト（必須）** |

### 集約のテストは業務ルールの仕様書です

サブテスト名を業務の言葉で書き、テスト結果の出力がそのまま業務ルールの一覧になるようにします。

```
--- PASS: TestInventory_Hold/空きがあれば確保できる
--- PASS: TestInventory_Hold/満室のときは確保できない
--- PASS: TestInventory_Hold/販売停止中は確保できない
```

### テスト実行

```bash
go test -race -count=1 ./...      # 全テスト
go test -short ./...               # DB不要な高速テストのみ
```

---

## 15. 実装の順序

**設計を完璧にしてから実装するのではなく、動くものを早く作ります。**

| 段階 | 内容 |
|---|---|
| 1 | 施設情報（Accommodation, RoomType）の型定義・検証・永続化 |
| 2 | **在庫集約 + 並行テスト**（本プロジェクトの中核） |
| 3 | 予約集約 |
| 4 | 在庫と予約の連携（決済なし） |
| 5 | 決済連携 → プロセスマネージャーが必要になる |
| 6 | 空室検索 → CQRSと投影が必要になる |
| 7 | 負荷試験とpprofによる改善 |

**必要になってから作ること。** 使う相手がいない段階でドメインイベントや
プロセスマネージャーの仕組みを作っても、動作は変わらず、なぜそれがあるのか分からなくなります。

---

## 16. 将来の拡張予定

本書11章「設計を進化させる」の実践として、以下を予定しています。

### 料金プランの分離

現在、料金は在庫集約の一部として保持しています（1部屋タイプ = 1料金）。

食事付きプラン、連泊割引、曜日別料金といった要求が生じた場合、
料金は独立したライフサイクルと複雑なルールを持つようになります。
その時点で**補完 → 中核へ格上げし、料金プランを独立した集約として切り出します。**

**分割に備え、料金の計算ロジックは値オブジェクトとして切り出しておきます。**

### スケール戦略

在庫集約が「部屋タイプ × 日付」という小さな単位であり、
他の集約と強い整合性を要求しない設計のため、**宿単位での水平分割が可能です。**

現在は単一DBで構成していますが、書き込みが逼迫した場合は
読み取りのレプリカ分離 → 宿単位のシャーディング、の順で対応できます。

集約の境界設計が、将来のスケール戦略を規定しています。

---

## 17. 分類の変化への対応

| 変化 | 対応 |
|---|---|
| 補完 → 中核 | **作り直す。** アクティブレコードでは業務ルールを守れなくなるため |
| 中核 → 補完 | **作り直さない。** 新機能の追加を止め、投資を減らすだけ |
| 中核 → 一般 | 既製品への置き換えを検討する |

分類が変わったときに変わるのは、主に「これから何に手をかけるか」であり、
既存コードの構造ではありません。
