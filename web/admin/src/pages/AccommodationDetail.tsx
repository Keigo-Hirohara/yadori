import { useEffect, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import {
  createRoomType,
  getAccommodation,
  listRoomTypes,
  type Accommodation,
  type RoomType,
} from "../api/client";
import { Button, Empty, ErrorBanner, Field, inputClass } from "../components/ui";

export default function AccommodationDetail() {
  const { accommodationId = "" } = useParams();
  const navigate = useNavigate();
  const [accommodation, setAccommodation] = useState<Accommodation | null>(null);
  const [roomTypes, setRoomTypes] = useState<RoomType[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [adding, setAdding] = useState(false);

  const load = () => {
    Promise.all([getAccommodation(accommodationId), listRoomTypes(accommodationId)])
      .then(([a, rt]) => {
        setAccommodation(a);
        setRoomTypes(rt);
      })
      .catch((e) => setError(e.message));
  };

  useEffect(load, [accommodationId]);

  if (adding) {
    return (
      <RoomTypeForm
        accommodationId={accommodationId}
        accommodationName={accommodation?.name ?? ""}
        onCancel={() => setAdding(false)}
        onCreated={() => {
          setAdding(false);
          load();
        }}
      />
    );
  }

  return (
    <div className="flex flex-col gap-[26px]">
      <header>
        <button className="btn btn-ghost -ml-[5px]" onClick={() => navigate("/")}>
          ← 宿一覧
        </button>
        <div className="mt-2 flex items-end gap-5">
          <h2 className="min-w-0 flex-1 text-[32px]">{accommodation?.name ?? "…"}</h2>
        </div>
        <ErrorBanner message={error} />
        {accommodation && (
          <div className="mt-[18px] flex flex-wrap gap-[34px] text-[13px]">
            <Meta label="電話番号" value={accommodation.phoneNumber} />
            <Meta label="郵便番号" value={accommodation.postalCode} />
            <Meta
              label="所在地"
              value={`${accommodation.prefecture}${accommodation.city}${accommodation.streetAddress} ${accommodation.building}`}
            />
          </div>
        )}
      </header>

      <section>
        <div className="mb-2.5 flex items-end gap-5">
          <h3 className="flex-1 text-[20px]">部屋タイプ</h3>
          <Button onClick={() => setAdding(true)}>部屋タイプを追加</Button>
        </div>

        {roomTypes.length === 0 ? (
          <Empty>まだ部屋タイプが登録されていません</Empty>
        ) : (
          <table className="table">
            <thead>
              <tr>
                <th style={{ width: "34%" }}>部屋タイプ名</th>
                <th style={{ width: "10%" }}>定員</th>
                <th style={{ width: "14%" }}>専用風呂</th>
                <th style={{ width: "14%" }}>バルコニー</th>
                <th style={{ width: "28%" }} />
              </tr>
            </thead>
            <tbody>
              {roomTypes.map((rt) => (
                <tr key={rt.roomTypeId}>
                  <td className="text-[15px]">{rt.name}</td>
                  <td>{rt.capacity}名</td>
                  <td>{rt.hasPrivateBath ? "あり" : "—"}</td>
                  <td>{rt.hasBalcony ? "あり" : "—"}</td>
                  <td className="text-right">
                    <button
                      className="btn btn-ghost whitespace-nowrap"
                      onClick={() => navigate(`/room-types/${rt.roomTypeId}/inventories`)}
                    >
                      在庫カレンダーを開く →
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </section>
    </div>
  );
}

function Meta({ label, value }: { label: string; value: string }) {
  return (
    <div>
      <div className="stat-label">{label}</div>
      <div className="mt-[3px]">{value}</div>
    </div>
  );
}

function RoomTypeForm({
  accommodationId,
  accommodationName,
  onCancel,
  onCreated,
}: {
  accommodationId: string;
  accommodationName: string;
  onCancel: () => void;
  onCreated: () => void;
}) {
  const [name, setName] = useState("");
  const [capacity, setCapacity] = useState(2);
  const [hasPrivateBath, setHasPrivateBath] = useState(false);
  const [hasBalcony, setHasBalcony] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const submit = async () => {
    setError(null);
    try {
      await createRoomType(accommodationId, { name, capacity, hasPrivateBath, hasBalcony });
      onCreated();
    } catch (e) {
      setError((e as Error).message);
    }
  };

  return (
    <div className="flex max-w-[560px] flex-col gap-5">
      <div>
        <button className="btn btn-ghost -ml-[5px]" onClick={onCancel}>
          ← {accommodationName}
        </button>
        <h2 className="mt-2 text-[30px]">部屋タイプを追加</h2>
      </div>
      <ErrorBanner message={error} />

      <Field label="部屋タイプ名">
        <input className={inputClass} value={name} onChange={(e) => setName(e.target.value)} />
      </Field>
      <Field label="定員" hint="1名以上で入力してください。" className="max-w-[200px]">
        <input type="number" min={1} className={inputClass} value={capacity} onChange={(e) => setCapacity(Number(e.target.value))} />
      </Field>
      <Field label="専用風呂">
        <Toggle value={hasPrivateBath} onChange={setHasPrivateBath} />
      </Field>
      <Field label="バルコニー">
        <Toggle value={hasBalcony} onChange={setHasBalcony} />
      </Field>

      <div className="flex gap-2.5">
        <Button onClick={submit}>追加する</Button>
        <Button variant="secondary" onClick={onCancel}>
          キャンセル
        </Button>
      </div>
    </div>
  );
}

function Toggle({ value, onChange }: { value: boolean; onChange: (v: boolean) => void }) {
  return (
    <div className="seg">
      <button type="button" className="seg-opt" aria-pressed={value} onClick={() => onChange(true)}>
        あり
      </button>
      <button type="button" className="seg-opt" aria-pressed={!value} onClick={() => onChange(false)}>
        なし
      </button>
    </div>
  );
}
