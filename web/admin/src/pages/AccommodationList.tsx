import { useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { listAccommodations, type Accommodation } from "../api/client";
import { Empty, ErrorBanner, PageHeader } from "../components/ui";

export default function AccommodationList() {
  const [accommodations, setAccommodations] = useState<Accommodation[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const navigate = useNavigate();

  useEffect(() => {
    listAccommodations()
      .then(setAccommodations)
      .catch((e) => setError(e.message))
      .finally(() => setLoading(false));
  }, []);

  return (
    <div className="flex flex-col gap-[22px]">
      <PageHeader
        kicker="Properties"
        title="宿一覧"
        lead="登録済みの宿と、それぞれの部屋タイプ・在庫を管理します。"
        action={
          <Link to="/accommodations/new" className="btn btn-primary">
            宿を登録
          </Link>
        }
      />
      <ErrorBanner message={error} />

      {loading ? (
        <Empty>読み込み中…</Empty>
      ) : accommodations.length === 0 ? (
        <Empty>まだ宿が登録されていません</Empty>
      ) : (
        <div className="grid gap-[14px]" style={{ gridTemplateColumns: "repeat(auto-fill, minmax(300px, 1fr))" }}>
          {accommodations.map((a) => (
            <div
              key={a.accommodationId}
              className="card elev-sm cursor-pointer"
              style={{ gap: 8, padding: "16px 18px" }}
              onClick={() => navigate(`/accommodations/${a.accommodationId}`)}
            >
              <div className="card-kicker">{a.prefecture}</div>
              <div className="card-title text-[19px]">{a.name}</div>
              <div className="text-[13px] leading-[1.65] text-[var(--color-neutral-700)]">
                {a.city}
                {a.streetAddress} {a.building}
              </div>
              <div className="mt-1 text-[12px] text-[var(--color-neutral-600)]">TEL {a.phoneNumber}</div>
            </div>
          ))}
        </div>
      )}
    </div>
  );
}
