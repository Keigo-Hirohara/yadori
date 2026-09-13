import { useEffect, useState } from "react";
import { useNavigate, useParams, useSearchParams } from "react-router-dom";
import { book, createBooker, getRoomType, type Guest, type RoomType } from "../api/client";
import { currentBookerId, rememberBookerId } from "../booker";
import { Button, ErrorBanner, Field, inputClass } from "../components/ui";
import { prefectures } from "../prefectures";

const fmt = (s: string) => {
  const [y, m, d] = s.split("-");
  return `${y}年${Number(m)}月${Number(d)}日`;
};

export default function BookingNew() {
  const { roomTypeId = "" } = useParams();
  const [params] = useSearchParams();
  const navigate = useNavigate();

  const checkin = params.get("checkin") ?? "";
  const checkout = params.get("checkout") ?? "";
  const guestCount = Number(params.get("guests") ?? 1);

  const [roomType, setRoomType] = useState<RoomType | null>(null);
  const [guests, setGuests] = useState<Guest[]>(
    Array.from({ length: guestCount }, () => ({ firstName: "", lastName: "" })),
  );
  const [booker, setBooker] = useState({
    firstName: "",
    lastName: "",
    phoneNumber: "",
    postalCode: "",
    prefecture: "",
    city: "",
    streetAddress: "",
    building: "",
  });
  const [agreed, setAgreed] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);

  const registered = currentBookerId();

  useEffect(() => {
    getRoomType(roomTypeId).then(setRoomType).catch((e) => setError(e.message));
  }, [roomTypeId]);

  const updateGuest = (index: number, key: keyof Guest, value: string) =>
    setGuests((gs) => gs.map((g, i) => (i === index ? { ...g, [key]: value } : g)));

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    setError(null);
    try {
      let bookerId = registered;
      if (!bookerId) {
        const created = await createBooker(booker);
        rememberBookerId(created.bookerId);
        bookerId = created.bookerId;
      }
      const booking = await book({ bookerId, roomTypeId, checkinDate: checkin, checkoutDate: checkout, guests });
      navigate(`/bookings/${booking.bookingId}/payment`);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="mx-auto max-w-[860px] px-6 pt-12 pb-32">
      <h1 className="mt-6 mb-2.5 text-[26px]">予約情報の入力</h1>
      <p className="text-muted mb-16 text-[13px]">
        {roomType?.name ?? "…"} ／ {fmt(checkin)} 〜 {fmt(checkout)} ／ {guestCount}名
      </p>
      <ErrorBanner message={error} />

      <form onSubmit={submit}>
        <h3 className="mb-1.5 text-[20px]">宿泊者</h3>
        <p className="mb-6 text-[13px] text-[var(--color-accent-2-700)]">
          宿泊されるお客さま全員分のお名前が必要です。後からの入力はできません。
        </p>
        <div className="flex max-w-[560px] flex-col gap-[26px]">
          {guests.map((g, i) => (
            <div key={i}>
              <div className="mb-2 text-[13px]">
                宿泊者 {i + 1}
                {i === 0 && "（代表者）"}
              </div>
              <div className="grid gap-4" style={{ gridTemplateColumns: "repeat(auto-fit, minmax(160px, 1fr))" }}>
                <Field label="姓">
                  <input className={inputClass} value={g.lastName} onChange={(e) => updateGuest(i, "lastName", e.target.value)} />
                </Field>
                <Field label="名">
                  <input className={inputClass} value={g.firstName} onChange={(e) => updateGuest(i, "firstName", e.target.value)} />
                </Field>
              </div>
            </div>
          ))}
        </div>

        {!registered && (
          <>
            <h3 className="mt-[52px] mb-1.5 text-[20px]">予約者さまの情報</h3>
            <p className="text-muted mb-6 text-[13px]">はじめてご利用の方は、こちらの入力がそのまま会員登録になります。</p>
            <div className="grid max-w-[560px] gap-5" style={{ gridTemplateColumns: "repeat(auto-fit, minmax(220px, 1fr))" }}>
              <Field label="姓">
                <input className={inputClass} value={booker.lastName} onChange={(e) => setBooker({ ...booker, lastName: e.target.value })} />
              </Field>
              <Field label="名">
                <input className={inputClass} value={booker.firstName} onChange={(e) => setBooker({ ...booker, firstName: e.target.value })} />
              </Field>
              <Field label="電話番号" hint="ハイフンなし">
                <input type="tel" className={inputClass} value={booker.phoneNumber} onChange={(e) => setBooker({ ...booker, phoneNumber: e.target.value })} />
              </Field>
              <Field label="郵便番号" hint="ハイフンなしの7桁">
                <input className={inputClass} value={booker.postalCode} onChange={(e) => setBooker({ ...booker, postalCode: e.target.value })} />
              </Field>
              <Field label="都道府県">
                <select className={inputClass} value={booker.prefecture} onChange={(e) => setBooker({ ...booker, prefecture: e.target.value })}>
                  <option value="">選択してください</option>
                  {prefectures.map((p) => (
                    <option key={p} value={p}>
                      {p}
                    </option>
                  ))}
                </select>
              </Field>
              <Field label="市区町村">
                <input className={inputClass} value={booker.city} onChange={(e) => setBooker({ ...booker, city: e.target.value })} />
              </Field>
              <Field label="番地" className="col-span-full">
                <input className={inputClass} value={booker.streetAddress} onChange={(e) => setBooker({ ...booker, streetAddress: e.target.value })} />
              </Field>
            </div>
          </>
        )}

        <label className="radio mt-5">
          <input type="checkbox" checked={agreed} onChange={(e) => setAgreed(e.target.checked)} />
          <span className="dot" />
          利用規約とキャンセル規定に同意します
        </label>

        <div className="notice notice-accent mt-8 max-w-[560px]">
          ご予約後、この部屋が<strong>30分間</strong>だけ確保されます。時間内にお支払いが完了しない場合、予約は自動的に取り消されます。
        </div>

        <div className="mt-11 flex flex-wrap gap-[14px]">
          <Button type="submit" className="h-12 px-8 text-[16px]" disabled={saving || !agreed}>
            {saving ? "処理中…" : "この内容で予約する"}
          </Button>
          <Button variant="secondary" className="h-12" onClick={() => navigate(-1)}>
            もどる
          </Button>
        </div>
      </form>
    </div>
  );
}
