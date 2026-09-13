import { useState } from "react";
import { useNavigate } from "react-router-dom";
import { createAccommodation, type AccommodationInput } from "../api/client";
import { Button, ErrorBanner, Field, inputClass } from "../components/ui";
import { prefectures } from "../prefectures";

const empty: AccommodationInput = {
  name: "",
  phoneNumber: "",
  postalCode: "",
  prefecture: "",
  city: "",
  streetAddress: "",
  building: "",
};

export default function AccommodationNew() {
  const [form, setForm] = useState(empty);
  const [error, setError] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const navigate = useNavigate();

  const update = (key: keyof AccommodationInput) => (value: string) =>
    setForm((f) => ({ ...f, [key]: value }));

  const submit = async (e: React.FormEvent) => {
    e.preventDefault();
    setSaving(true);
    setError(null);
    try {
      const created = await createAccommodation(form);
      navigate(`/accommodations/${created.accommodationId}`);
    } catch (e) {
      setError((e as Error).message);
    } finally {
      setSaving(false);
    }
  };

  return (
    <div className="flex max-w-[720px] flex-col gap-5">
      <div>
        <button className="btn btn-ghost -ml-[5px]" onClick={() => navigate("/")}>
          ← 宿一覧
        </button>
        <h2 className="mt-2 text-[30px]">宿を登録</h2>
        <p className="mt-1.5 text-[13px] text-[var(--color-neutral-700)]">
          登録後、部屋タイプを追加すると在庫カレンダーが使えます。
        </p>
      </div>
      <ErrorBanner message={error} />

      <form onSubmit={submit} className="flex flex-col gap-5">
        <div className="grid grid-cols-2 gap-x-5 gap-y-4">
          <Field label="宿名" className="col-span-2">
            <input className={inputClass} value={form.name} onChange={(e) => update("name")(e.target.value)} />
          </Field>
          <Field label="電話番号" hint="ハイフンなしの数字で入力してください。">
            <input className={inputClass} value={form.phoneNumber} onChange={(e) => update("phoneNumber")(e.target.value)} />
          </Field>
          <Field label="郵便番号" hint="ハイフンなしの7桁で入力してください。">
            <input className={inputClass} style={{ maxWidth: 180 }} value={form.postalCode} onChange={(e) => update("postalCode")(e.target.value)} />
          </Field>
          <Field label="都道府県">
            <select className={inputClass} value={form.prefecture} onChange={(e) => update("prefecture")(e.target.value)}>
              <option value="">選択してください</option>
              {prefectures.map((p) => (
                <option key={p} value={p}>
                  {p}
                </option>
              ))}
            </select>
          </Field>
          <Field label="市区町村">
            <input className={inputClass} value={form.city} onChange={(e) => update("city")(e.target.value)} />
          </Field>
          <Field label="番地">
            <input className={inputClass} value={form.streetAddress} onChange={(e) => update("streetAddress")(e.target.value)} />
          </Field>
          <Field label="建物名">
            <input className={inputClass} placeholder="任意" value={form.building} onChange={(e) => update("building")(e.target.value)} />
          </Field>
        </div>
        <div className="mt-1.5 flex gap-2.5">
          <Button type="submit" disabled={saving}>
            {saving ? "登録中…" : "この内容で登録"}
          </Button>
          <Button variant="secondary" onClick={() => navigate("/")}>
            キャンセル
          </Button>
        </div>
      </form>
    </div>
  );
}
