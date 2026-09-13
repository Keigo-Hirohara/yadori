import { useCallback, useEffect, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import {
  ApiError,
  changeFee,
  changeQuantity,
  closeInventory,
  getRoomType,
  listInventories,
  registerInventory,
  reopenInventory,
  type Inventory,
  type RoomType,
} from "../api/client";
import { Button, ErrorBanner, Field, Modal, Stat, inputClass, yen } from "../components/ui";

const iso = (d: Date) =>
  `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;

function monthGrid(year: number, month: number): (Date | null)[] {
  const first = new Date(year, month, 1);
  const days = new Date(year, month + 1, 0).getDate();
  const cells: (Date | null)[] = Array(first.getDay()).fill(null);
  for (let d = 1; d <= days; d++) cells.push(new Date(year, month, d));
  while (cells.length % 7 !== 0) cells.push(null);
  return cells;
}

const DOW = ["日", "月", "火", "水", "木", "金", "土"];

export default function InventoryCalendar() {
  const { roomTypeId = "" } = useParams();
  const navigate = useNavigate();
  const [roomType, setRoomType] = useState<RoomType | null>(null);
  const [cursor, setCursor] = useState(() => {
    const now = new Date();
    return new Date(now.getFullYear(), now.getMonth(), 1);
  });
  const [inventories, setInventories] = useState<Map<string, Inventory>>(new Map());
  const [error, setError] = useState<string | null>(null);
  const [registering, setRegistering] = useState<{ from: string; to: string } | null>(null);
  const [editing, setEditing] = useState<{ date: string; inventory: Inventory } | null>(null);
  const [drag, setDrag] = useState<{ a: string; b: string } | null>(null);

  const year = cursor.getFullYear();
  const month = cursor.getMonth();

  const load = useCallback(() => {
    const from = iso(new Date(year, month, 1));
    const to = iso(new Date(year, month + 1, 1));
    listInventories(roomTypeId, from, to)
      .then((list) => setInventories(new Map(list.map((i) => [i.date, i]))))
      .catch((e) => setError(e.message));
  }, [roomTypeId, year, month]);

  useEffect(() => {
    getRoomType(roomTypeId).then(setRoomType).catch((e) => setError(e.message));
  }, [roomTypeId]);

  useEffect(load, [load]);

  useEffect(() => {
    if (!drag) return;
    const finish = () => {
      const [lo, hi] = [drag.a, drag.b].sort();
      setDrag(null);
      setRegistering({ from: lo, to: hi });
    };
    window.addEventListener("mouseup", finish);
    return () => window.removeEventListener("mouseup", finish);
  }, [drag]);

  const cells = monthGrid(year, month);
  const stat = { registered: 0, full: 0, closed: 0, none: 0 };
  for (const date of cells) {
    if (!date) continue;
    const inv = inventories.get(iso(date));
    if (!inv) stat.none++;
    else if (inv.isClosed) stat.closed++;
    else {
      stat.registered++;
      if (inv.available <= 0) stat.full++;
    }
  }

  const [selLo, selHi] = drag ? [drag.a, drag.b].sort() : [null, null];

  return (
    <div className="flex flex-col gap-4 select-none">
      <header className="flex flex-wrap items-end gap-5">
        <div className="min-w-[240px] flex-1">
          <div className="kicker">Inventory</div>
          <h2 className="mt-1 text-[30px]">在庫カレンダー</h2>
          <div className="mt-1.5 text-[13px] text-[var(--color-neutral-700)]">
            {roomType ? (
              <button className="btn btn-ghost -ml-[5px]" onClick={() => navigate(`/accommodations/${roomType.accommodationId}`)}>
                ← {roomType.name}
              </button>
            ) : (
              "…"
            )}
          </div>
        </div>
        <Button onClick={() => setRegistering({ from: iso(new Date()), to: iso(new Date()) })}>
          在庫を一括登録
        </Button>
      </header>
      <ErrorBanner message={error} />

      <div className="flex flex-wrap items-center gap-[14px]">
        <div className="flex items-center gap-1.5">
          <button className="btn btn-secondary btn-icon" aria-label="前の月" onClick={() => setCursor(new Date(year, month - 1, 1))}>
            ‹
          </button>
          <div className="min-w-[150px] text-center text-[21px] font-semibold" style={{ fontFamily: "var(--font-heading)" }}>
            {year}年{month + 1}月
          </div>
          <button className="btn btn-secondary btn-icon" aria-label="次の月" onClick={() => setCursor(new Date(year, month + 1, 1))}>
            ›
          </button>
        </div>
        <div className="ml-auto flex items-center gap-[26px] text-[12px] text-[var(--color-neutral-700)]">
          <span>登録済 {stat.registered}日</span>
          <span>満室 {stat.full}日</span>
          <span>停止中 {stat.closed}日</span>
          <span>未登録 {stat.none}日</span>
        </div>
      </div>

      <div className="text-[12px] text-[var(--color-neutral-600)]">
        マスをドラッグすると複数日をまとめて登録できます。登録済みの日をクリックすると1日分を編集します。
      </div>

      <div className="grid grid-cols-7 gap-[5px]">
        {DOW.map((d, i) => (
          <div
            key={d}
            className="pb-1 text-center text-[11px] tracking-[0.08em]"
            style={{ color: i === 0 ? "var(--color-accent-2-700)" : i === 6 ? "var(--color-accent-700)" : "var(--color-neutral-500)" }}
          >
            {d}
          </div>
        ))}
        {cells.map((date, i) => {
          if (!date) return <div key={`e${i}`} className="min-h-[94px]" />;
          const key = iso(date);
          const inv = inventories.get(key);
          const selected = selLo !== null && key >= selLo && key <= selHi!;
          return (
            <DayCell
              key={key}
              date={date}
              inventory={inv}
              selected={selected}
              onMouseDown={() => setDrag({ a: key, b: key })}
              onMouseEnter={() => drag && setDrag({ a: drag.a, b: key })}
              onClick={() => inv && !drag && setEditing({ date: key, inventory: inv })}
            />
          );
        })}
      </div>

      <div className="mt-1 flex flex-wrap gap-[18px] text-[12px] text-[var(--color-neutral-700)]">
        <Legend swatch={{ background: "var(--color-surface)", boxShadow: "var(--shadow-sm)" }}>販売中</Legend>
        <Legend swatch={{ background: "var(--color-accent-2-100)" }}>満室（残0）</Legend>
        <Legend swatch={{ background: "var(--color-neutral-200)" }}>販売停止中</Legend>
        <Legend swatch={{ border: "1px dashed var(--color-neutral-400)" }}>未登録</Legend>
      </div>

      {registering && (
        <RegisterModal
          roomTypeId={roomTypeId}
          roomTypeName={roomType?.name ?? ""}
          initialFrom={registering.from}
          initialTo={registering.to}
          onClose={() => setRegistering(null)}
          onDone={() => {
            setRegistering(null);
            load();
          }}
        />
      )}

      {editing && (
        <EditModal
          roomTypeId={roomTypeId}
          roomTypeName={roomType?.name ?? ""}
          date={editing.date}
          inventory={editing.inventory}
          onClose={() => setEditing(null)}
          onDone={() => {
            setEditing(null);
            load();
          }}
        />
      )}
    </div>
  );
}

function Legend({ swatch, children }: { swatch: React.CSSProperties; children: React.ReactNode }) {
  return (
    <span className="flex items-center gap-[7px]">
      <span style={{ width: 13, height: 13, ...swatch }} />
      {children}
    </span>
  );
}

function DayCell({
  date,
  inventory,
  selected,
  onMouseDown,
  onMouseEnter,
  onClick,
}: {
  date: Date;
  inventory?: Inventory;
  selected: boolean;
  onMouseDown: () => void;
  onMouseEnter: () => void;
  onClick: () => void;
}) {
  const dow = date.getDay();
  const full = !!inventory && !inventory.isClosed && inventory.available <= 0;

  let bg = "var(--color-surface)";
  let ink = "var(--color-text)";
  let border = "1px solid transparent";
  let shadow = "var(--shadow-sm)";
  if (!inventory) {
    bg = "transparent";
    ink = "var(--color-neutral-500)";
    border = "1px dashed var(--color-neutral-400)";
    shadow = "none";
  } else if (inventory.isClosed) {
    bg = "var(--color-neutral-200)";
    ink = "var(--color-neutral-700)";
    shadow = "none";
  } else if (full) {
    bg = "var(--color-accent-2-100)";
  }

  const dowInk = dow === 0 ? "var(--color-accent-2-700)" : dow === 6 ? "var(--color-accent-700)" : ink;
  const badge = !inventory ? "未登録" : inventory.isClosed ? "停止中" : full ? "満室" : null;
  const badgeStyle: React.CSSProperties = !inventory
    ? { color: "var(--color-neutral-500)" }
    : inventory.isClosed
      ? { background: "var(--color-neutral-400)", color: "#fff" }
      : { background: "var(--color-accent-2-500)", color: "#fff" };

  return (
    <div
      onMouseDown={onMouseDown}
      onMouseEnter={onMouseEnter}
      onClick={onClick}
      className="flex min-h-[94px] cursor-pointer flex-col gap-0.5 rounded-[var(--radius-md)] px-[9px] py-[7px]"
      style={{
        background: selected ? "var(--color-accent-100)" : bg,
        color: ink,
        border,
        boxShadow: shadow,
        outline: selected ? "2px solid var(--color-accent)" : undefined,
        outlineOffset: -2,
      }}
    >
      <div className="flex items-baseline gap-1.5">
        <span className="text-[14px] font-semibold" style={{ fontFamily: "var(--font-heading)", color: dowInk }}>
          {date.getDate()}
        </span>
        {badge && (
          <span className="rounded-[2px] px-1.5 py-px text-[10px] tracking-[0.04em]" style={badgeStyle}>
            {badge}
          </span>
        )}
      </div>
      {inventory && (
        <>
          <div
            className="mt-auto text-[16px] font-semibold tracking-[-0.01em]"
            style={{ fontFamily: "var(--font-heading)", color: inventory.isClosed ? "var(--color-neutral-600)" : undefined }}
          >
            {yen(inventory.fee)}
          </div>
          <div className="text-[12px]" style={{ color: full ? "var(--color-accent-2-800)" : "var(--color-neutral-600)" }}>
            残{inventory.available} / {inventory.quantityAvailable}
          </div>
        </>
      )}
    </div>
  );
}

function RegisterModal({
  roomTypeId,
  roomTypeName,
  initialFrom,
  initialTo,
  onClose,
  onDone,
}: {
  roomTypeId: string;
  roomTypeName: string;
  initialFrom: string;
  initialTo: string;
  onClose: () => void;
  onDone: () => void;
}) {
  const [from, setFrom] = useState(initialFrom);
  const [to, setTo] = useState(initialTo);
  const [quantity, setQuantity] = useState(3);
  const [fee, setFee] = useState(15000);
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  const dates = (() => {
    const list: string[] = [];
    for (let d = new Date(from); d <= new Date(to); d = new Date(d.getFullYear(), d.getMonth(), d.getDate() + 1)) {
      list.push(iso(d));
    }
    return list;
  })();

  const submit = async () => {
    setError(null);
    setBusy(true);
    const results = await Promise.all(
      dates.map(async (date) => {
        try {
          await registerInventory(roomTypeId, { date, quantity, fee });
          return "created";
        } catch (e) {
          if (e instanceof ApiError && e.code === "INVENTORY_ALREADY_REGISTERED") {
            try {
              await changeQuantity(roomTypeId, date, quantity);
              await changeFee(roomTypeId, date, fee);
              return "updated";
            } catch {
              return "failed";
            }
          }
          return "failed";
        }
      }),
    );
    setBusy(false);
    const failed = results.filter((r) => r === "failed").length;
    if (failed > 0) {
      setError(`${failed}日分の登録に失敗しました。残りは反映されています。`);
      return;
    }
    onDone();
  };

  return (
    <Modal title="在庫を登録" subtitle={roomTypeName} onClose={onClose} width={480}>
      <ErrorBanner message={error} />
      <div className="grid items-end gap-2.5" style={{ gridTemplateColumns: "1fr auto 1fr" }}>
        <Field label="開始日">
          <input type="date" className={inputClass} value={from} onChange={(e) => setFrom(e.target.value)} />
        </Field>
        <div className="pb-[11px] text-[var(--color-neutral-600)]">〜</div>
        <Field label="終了日">
          <input type="date" className={inputClass} value={to} onChange={(e) => setTo(e.target.value)} />
        </Field>
      </div>
      <div className="grid grid-cols-2 gap-[14px]">
        <Field label="販売枠数">
          <input type="number" min={0} className={inputClass} value={quantity} onChange={(e) => setQuantity(Number(e.target.value))} />
        </Field>
        <Field label="1泊あたりの料金（円）">
          <input type="number" min={0} step={500} className={inputClass} value={fee} onChange={(e) => setFee(Number(e.target.value))} />
        </Field>
      </div>
      <div className="notice notice-accent rounded-[var(--radius-md)] py-3 px-[14px]">
        {dates.length}日分を、{quantity}枠・{yen(fee)} で登録します。
      </div>
      <div className="-mt-1 text-[12px] text-[var(--color-neutral-600)]">
        すでに在庫がある日は、枠数と料金を変更します。確保済みの予約はそのまま残ります。
      </div>
      <div className="dialog-actions">
        <Button variant="secondary" onClick={onClose}>
          キャンセル
        </Button>
        <Button onClick={submit} disabled={busy || dates.length === 0}>
          {busy ? "登録中…" : "登録する"}
        </Button>
      </div>
    </Modal>
  );
}

function EditModal({
  roomTypeId,
  roomTypeName,
  date,
  inventory,
  onClose,
  onDone,
}: {
  roomTypeId: string;
  roomTypeName: string;
  date: string;
  inventory: Inventory;
  onClose: () => void;
  onDone: () => void;
}) {
  const [quantity, setQuantity] = useState(inventory.quantityAvailable);
  const [fee, setFee] = useState(inventory.fee);
  const [error, setError] = useState<string | null>(null);

  const run = async (action: () => Promise<unknown>) => {
    setError(null);
    try {
      await action();
      onDone();
    } catch (e) {
      setError((e as Error).message);
    }
  };

  const save = () =>
    run(async () => {
      if (quantity !== inventory.quantityAvailable) await changeQuantity(roomTypeId, date, quantity);
      if (fee !== inventory.fee) await changeFee(roomTypeId, date, fee);
    });

  const [y, m, d] = date.split("-");

  return (
    <Modal title={`${y}年${Number(m)}月${Number(d)}日 の在庫`} subtitle={roomTypeName} onClose={onClose}>
      <ErrorBanner message={error} />
      <div className="flex gap-[30px] py-1">
        <Stat label="確保済み" value={`${inventory.heldCount}件`} />
        <Stat label="残数" value={inventory.available} />
        <Stat label="販売状態" value={inventory.isClosed ? "停止中" : "販売中"} />
      </div>
      <div className="grid grid-cols-2 gap-[14px]">
        <Field label="販売枠数">
          <input type="number" min={inventory.heldCount} className={inputClass} value={quantity} onChange={(e) => setQuantity(Number(e.target.value))} />
        </Field>
        <Field label="1泊あたりの料金（円）">
          <input type="number" min={0} step={500} className={inputClass} value={fee} onChange={(e) => setFee(Number(e.target.value))} />
        </Field>
      </div>
      {inventory.heldCount > 0 && (
        <div className="text-[12px] text-[var(--color-accent-2-700)]">
          確保済みの{inventory.heldCount}件を下回る枠数には変更できません。
        </div>
      )}
      <div className="mt-0.5 flex flex-col items-start gap-2">
        <Button
          variant="secondary"
          onClick={() => run(() => (inventory.isClosed ? reopenInventory(roomTypeId, date) : closeInventory(roomTypeId, date)))}
        >
          {inventory.isClosed ? "販売を再開する" : "販売を停止する"}
        </Button>
        <span className="text-[12px] leading-[1.5] text-[var(--color-neutral-600)]">
          販売を停止しても枠数と確保済みの予約は残り、新規受付だけ止まります。
        </span>
      </div>
      <div className="dialog-actions">
        <Button variant="secondary" onClick={onClose}>
          キャンセル
        </Button>
        <Button onClick={save}>変更を保存</Button>
      </div>
    </Modal>
  );
}
