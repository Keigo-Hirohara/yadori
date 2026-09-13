import { useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { searchRoomTypes, type SearchResult } from "../api/client";
import { Button, Empty, ErrorBanner, Field, inputClass, yen } from "../components/ui";
import { prefectures } from "../prefectures";

const iso = (d: Date) =>
  `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, "0")}-${String(d.getDate()).padStart(2, "0")}`;

const daysFromToday = (days: number) => {
  const d = new Date();
  d.setDate(d.getDate() + days);
  return iso(d);
};

export default function Search() {
  const [prefecture, setPrefecture] = useState("");
  const [checkin, setCheckin] = useState(daysFromToday(1));
  const [checkout, setCheckout] = useState(daysFromToday(2));
  const [guests, setGuests] = useState(2);
  const [maxFee, setMaxFee] = useState("");
  const navigate = useNavigate();

  const submit = (e: React.FormEvent) => {
    e.preventDefault();
    const params = new URLSearchParams({ checkin, checkout, guests: String(guests) });
    if (prefecture) params.set("prefecture", prefecture);
    if (maxFee) params.set("maxFee", maxFee);
    navigate(`/results?${params}`);
  };

  return (
    <div className="relative mt-2">
      <div className="halftone absolute inset-0" />
      <div className="relative mx-auto flex max-w-[1080px] justify-center px-6 pt-[120px] pb-[140px]">
        <form onSubmit={submit} className="w-[min(660px,100%)] bg-[var(--color-bg)] px-14 pt-14 pb-[52px]">
          <h1 className="mb-[42px] text-[31px] leading-[1.75]">
            日本の宿を、
            <br />
            日程と人数で。
          </h1>
          <div className="grid gap-x-8 gap-y-[30px]" style={{ gridTemplateColumns: "repeat(auto-fit, minmax(180px, 1fr))" }}>
            <Field label="都道府県" className="col-span-full">
              <select className={inputClass} value={prefecture} onChange={(e) => setPrefecture(e.target.value)}>
                <option value="">指定しない（全国から探す）</option>
                {prefectures.map((p) => (
                  <option key={p} value={p}>
                    {p}
                  </option>
                ))}
              </select>
            </Field>
            <Field label="チェックイン">
              <input type="date" className={inputClass} value={checkin} onChange={(e) => setCheckin(e.target.value)} />
            </Field>
            <Field label="チェックアウト">
              <input type="date" className={inputClass} value={checkout} onChange={(e) => setCheckout(e.target.value)} />
            </Field>
            <Field label="宿泊人数">
              <input type="number" min={1} className={inputClass} value={guests} onChange={(e) => setGuests(Number(e.target.value))} />
            </Field>
            <Field label="予算上限（宿泊日数の合計）">
              <input type="number" className={inputClass} value={maxFee} placeholder="上限なし" onChange={(e) => setMaxFee(e.target.value)} />
            </Field>
          </div>

          <Button type="submit" className="btn-block mt-7 h-12 text-[16px]">
            宿を検索する
          </Button>
        </form>
      </div>
      <TonightAvailability />
    </div>
  );
}

type AccommodationGroup = {
  accommodationId: string;
  accommodationName: string;
  prefecture: string;
  city: string;
  roomTypes: SearchResult[];
};

const groupByAccommodation = (results: SearchResult[]): AccommodationGroup[] => {
  const groups = new Map<string, AccommodationGroup>();
  for (const r of results) {
    const g = groups.get(r.accommodationId) ?? {
      accommodationId: r.accommodationId,
      accommodationName: r.accommodationName,
      prefecture: r.prefecture,
      city: r.city,
      roomTypes: [],
    };
    g.roomTypes.push(r);
    groups.set(r.accommodationId, g);
  }
  return [...groups.values()];
};

function TonightAvailability() {
  const [groups, setGroups] = useState<AccommodationGroup[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const checkin = daysFromToday(0);
  const checkout = daysFromToday(1);

  useEffect(() => {
    searchRoomTypes({ checkin, checkout, guests: 1 })
      .then((results) => setGroups(groupByAccommodation(results)))
      .catch((e) => setError(e.message))
      .finally(() => setLoading(false));
  }, [checkin, checkout]);

  const bookingParams = new URLSearchParams({ checkin, checkout, guests: "1" });

  return (
    <section className="relative mx-auto max-w-[1080px] px-6 pb-32">
      <h2 className="mb-10 text-[26px]">今日泊まれる宿</h2>
      <ErrorBanner message={error} />

      {loading ? (
        <Empty>読み込み中…</Empty>
      ) : groups.length === 0 ? (
        <Empty>本日ご案内できる宿はありません</Empty>
      ) : (
        <div className="grid gap-6" style={{ gridTemplateColumns: "repeat(auto-fill, minmax(300px, 1fr))" }}>
          {groups.map((g) => (
            <div key={g.accommodationId} className="card gap-4 p-6">
              <div>
                <div className="card-title text-[19px]">{g.accommodationName}</div>
                <div className="text-muted mt-1 text-[13px]">
                  {g.prefecture} {g.city}
                </div>
              </div>
              <ul className="flex flex-col divide-y divide-[var(--color-divider)]">
                {g.roomTypes.map((r) => (
                  <li key={r.roomTypeId} className="flex items-baseline gap-3 py-2.5 text-[14px]">
                    <span className="min-w-0 flex-1 truncate">{r.roomTypeName}</span>
                    <span className="text-muted text-[12px]">定員{r.capacity}名</span>
                    <span className="text-[16px]">{yen(r.totalFee)}</span>
                    <Link to={`/room-types/${r.roomTypeId}/book?${bookingParams}`} className="btn btn-ghost whitespace-nowrap">
                      予約へ
                    </Link>
                  </li>
                ))}
              </ul>
            </div>
          ))}
        </div>
      )}
    </section>
  );
}
