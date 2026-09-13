import { useEffect, useState } from "react";
import { Link, useSearchParams } from "react-router-dom";
import { searchRoomTypes, type SearchResult } from "../api/client";
import { Empty, ErrorBanner, yen } from "../components/ui";

const fmt = (s: string) => {
  const [y, m, d] = s.split("-");
  return `${y}年${Number(m)}月${Number(d)}日`;
};

export default function SearchResults() {
  const [params] = useSearchParams();
  const [results, setResults] = useState<SearchResult[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);

  const checkin = params.get("checkin") ?? "";
  const checkout = params.get("checkout") ?? "";
  const guests = Number(params.get("guests") ?? 1);
  const prefecture = params.get("prefecture") ?? "";

  useEffect(() => {
    setLoading(true);
    searchRoomTypes({
      checkin,
      checkout,
      guests,
      prefecture: prefecture || undefined,
      maxFee: params.get("maxFee") ? Number(params.get("maxFee")) : undefined,
    })
      .then(setResults)
      .catch((e) => setError(e.message))
      .finally(() => setLoading(false));
  }, [params, checkin, checkout, guests, prefecture]);

  const bookingParams = new URLSearchParams({ checkin, checkout, guests: String(guests) });
  const nights = results[0]?.nights;

  return (
    <div className="mx-auto max-w-[1080px] px-6 pt-12 pb-32">
      <p className="text-muted mb-1.5 text-[13px]">
        {prefecture || "全国"} ／ {fmt(checkin)} 〜 {fmt(checkout)}
        {nights ? ` ${nights}泊` : ""} ／ {guests}名
      </p>
      <h1 className="mb-1.5 text-[26px]">
        {loading ? "検索中" : `検索結果 ${results.length}件`}
      </h1>
      <p className="text-muted mb-14 text-[12px]">表示は参考情報です。空室の確定はご予約時となります。</p>
      <ErrorBanner message={error} />

      {loading ? (
        <Empty>検索中…</Empty>
      ) : results.length === 0 ? (
        <Empty>条件に合う空室が見つかりませんでした</Empty>
      ) : (
        <div className="flex flex-col gap-7">
          {results.map((r) => (
            <Link
              key={r.roomTypeId}
              to={`/room-types/${r.roomTypeId}/book?${bookingParams}`}
              className="card flex-row flex-wrap gap-[30px] p-7 text-inherit no-underline"
            >
              <div className="halftone h-40 min-w-[180px] shrink-0 basis-[220px]" />
              <div className="flex min-w-[260px] flex-1 flex-col gap-1.5">
                <div className="card-kicker">{r.accommodationName}</div>
                <div className="card-title mt-0.5 text-[19px]">{r.roomTypeName}</div>
                <div className="text-muted text-[13px]">
                  {r.prefecture} {r.city}
                </div>
                <div className="mt-0.5 flex flex-wrap gap-4 text-[13px]">
                  <Amenity>定員 {r.capacity}名</Amenity>
                  {r.hasPrivateBath && <Amenity>専用風呂</Amenity>}
                  {r.hasBalcony && <Amenity>バルコニー</Amenity>}
                </div>
                <div className="mt-auto flex flex-wrap items-baseline gap-[14px] pt-2.5">
                  <span className="text-[22px]">{yen(Math.round(r.totalFee / r.nights))}</span>
                  <span className="text-muted text-[13px]">／ 1泊</span>
                  <span className="ml-auto text-[14px]">
                    {r.nights}泊合計 <strong className="text-[18px]">{yen(r.totalFee)}</strong>
                  </span>
                </div>
              </div>
            </Link>
          ))}
        </div>
      )}
    </div>
  );
}

function Amenity({ children }: { children: React.ReactNode }) {
  return (
    <span className="inline-flex items-center gap-[5px]">
      <span className="inline-block h-1.5 w-1.5 rounded-full bg-[var(--color-accent)]" />
      {children}
    </span>
  );
}
