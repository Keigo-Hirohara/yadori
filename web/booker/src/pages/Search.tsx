import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { Button, Field, inputClass } from "../components/ui";
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
          <h1 className="mb-[14px] text-[31px] leading-[1.75]">
            日本の宿を、
            <br />
            日程と人数で。
          </h1>
          <p className="text-muted mb-[42px] text-[14px]">
            前払い制。ご予約後、30分以内のお支払いで確定します。
          </p>

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
          <p className="text-muted mt-[14px] text-[12px]">
            空室状況は目安です。満室の場合、ご予約時にお受けできないことがあります。
          </p>
        </form>
      </div>
    </div>
  );
}
