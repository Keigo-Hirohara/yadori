import { useEffect, useState } from "react";
import { Link, useNavigate } from "react-router-dom";
import { listBookings, type BookingSummary } from "../api/client";
import { currentBookerId } from "../booker";
import { Empty, ErrorBanner, yen } from "../components/ui";
import { StatusTag } from "./BookingDetail";

const today = new Date().toISOString().slice(0, 10);

export default function BookingHistory() {
  const [bookings, setBookings] = useState<BookingSummary[]>([]);
  const [error, setError] = useState<string | null>(null);
  const [loading, setLoading] = useState(true);
  const bookerId = currentBookerId();

  useEffect(() => {
    if (!bookerId) {
      setLoading(false);
      return;
    }
    listBookings(bookerId)
      .then(setBookings)
      .catch((e) => setError(e.message))
      .finally(() => setLoading(false));
  }, [bookerId]);

  const upcoming = bookings.filter((b) => b.checkoutDate >= today);
  const past = bookings.filter((b) => b.checkoutDate < today);

  return (
    <div className="mx-auto max-w-[1080px] px-6 pt-12 pb-32">
      <h1 className="mt-6 mb-[52px] text-[26px]">予約履歴</h1>
      <ErrorBanner message={error} />

      {!bookerId ? (
        <Empty>
          まだご予約がありません。
          <Link to="/" className="ml-1 underline">
            宿を探す
          </Link>
        </Empty>
      ) : loading ? (
        <Empty>読み込み中…</Empty>
      ) : bookings.length === 0 ? (
        <Empty>予約はまだありません</Empty>
      ) : (
        <>
          <Section title="これからのご予約" rows={upcoming} />
          <Section title="過去のご予約" rows={past} />
        </>
      )}
    </div>
  );
}

function Section({ title, rows }: { title: string; rows: BookingSummary[] }) {
  const navigate = useNavigate();
  if (rows.length === 0) return null;
  return (
    <>
      <h3 className="text-muted mb-3 text-[14px] uppercase tracking-[0.1em]">{title}</h3>
      <table className="table mb-[52px]">
        <thead>
          <tr>
            <th>日程</th>
            <th>予約番号</th>
            <th className="text-right">合計</th>
            <th className="text-right">状態</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((b) => (
            <tr key={b.bookingId} className="cursor-pointer" onClick={() => navigate(`/bookings/${b.bookingId}`)}>
              <td className="whitespace-nowrap">
                {b.checkinDate.slice(5).replace("-", "/")} 〜 {b.checkoutDate.slice(5).replace("-", "/")}
                <br />
                <span className="text-muted text-[12px]">{b.checkinDate.slice(0, 4)}年</span>
              </td>
              <td className="tabular-nums text-[12px]">{b.bookingId}</td>
              <td className="text-right">
                {yen(b.totalFee)}
                {b.cancellationFee > 0 && (
                  <>
                    <br />
                    <span className="text-muted text-[12px]">キャンセル料 {yen(b.cancellationFee)}</span>
                  </>
                )}
              </td>
              <td className="text-right">
                <StatusTag status={b.status} />
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </>
  );
}
