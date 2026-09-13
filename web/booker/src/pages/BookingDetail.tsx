import { useEffect, useState } from "react";
import { Link, useParams } from "react-router-dom";
import { cancel, getBooking, statusLabel, type Booking, type BookingStatus } from "../api/client";
import { Button, ErrorBanner, Modal, yen } from "../components/ui";

const fmt = (s: string) => {
  const [y, m, d] = s.split("-");
  return `${y}年${Number(m)}月${Number(d)}日`;
};

const statusNote: Record<BookingStatus, string> = {
  temporary_hold: "お部屋を確保しています。30分以内にお支払いを完了してください。",
  processing_payment: "お支払いの結果を確認しています。",
  confirmed: "ご予約は確定しています。お待ちしております。",
  cancelled: "このご予約はキャンセルされています。",
};

export default function BookingDetail() {
  const { bookingId = "" } = useParams();
  const [booking, setBooking] = useState<Booking | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [confirming, setConfirming] = useState(false);
  const [cancelledFee, setCancelledFee] = useState<number | null>(null);

  const load = () => {
    getBooking(bookingId).then(setBooking).catch((e) => setError(e.message));
  };

  useEffect(load, [bookingId]);

  const doCancel = async () => {
    setError(null);
    try {
      const result = await cancel(bookingId);
      setCancelledFee(result.cancellationFee);
      setConfirming(false);
      load();
    } catch (e) {
      setError((e as Error).message);
    }
  };

  const cancellable = booking?.status !== "cancelled";

  return (
    <div className="mx-auto max-w-[860px] px-6 pt-12 pb-32">
      <div className="flex flex-wrap items-center gap-4">
        <h1 className="text-[26px]">予約詳細</h1>
        {booking && <StatusTag status={booking.status} />}
      </div>
      <p className="text-muted mb-10 text-[13px]">{booking ? statusNote[booking.status] : ""}</p>
      <ErrorBanner message={error} />

      {cancelledFee !== null && (
        <div className="notice notice-neutral mb-6 max-w-[560px]">
          予約をキャンセルしました。キャンセル料は <strong>{yen(cancelledFee)}</strong> です。
        </div>
      )}

      {booking && (
        <>
          <table className="table mb-11">
            <tbody>
              <tr>
                <td className="text-muted w-[150px]">予約番号</td>
                <td className="tabular-nums">{booking.bookingId}</td>
              </tr>
              <tr>
                <td className="text-muted">日程</td>
                <td>
                  {fmt(booking.checkinDate)} 〜 {fmt(booking.checkoutDate)} {booking.nights}泊
                </td>
              </tr>
              <tr>
                <td className="text-muted">宿泊者</td>
                <td>{booking.guests.map((g) => `${g.lastName} ${g.firstName} さま`).join(" ／ ")}</td>
              </tr>
              <tr>
                <td className="text-muted">合計金額</td>
                <td>
                  <strong className="text-[18px]">{yen(booking.totalFee)}</strong>
                </td>
              </tr>
            </tbody>
          </table>

          {booking.status === "temporary_hold" && (
            <div className="notice notice-accent mb-9 max-w-[560px]">
              まだお支払いが完了していません。
              <Link to={`/bookings/${bookingId}/payment`} className="ml-2 underline">
                お支払いに進む →
              </Link>
            </div>
          )}
        </>
      )}

      <h3 className="mb-[14px] text-[18px]">キャンセル規定</h3>
      <table className="table mb-9 max-w-[520px]">
        <thead>
          <tr>
            <th>取消日</th>
            <th className="text-right">キャンセル料</th>
          </tr>
        </thead>
        <tbody>
          {[
            ["宿泊日の7日前まで", "無料"],
            ["6日前 〜 3日前", "宿泊料金の50%"],
            ["2日前 〜 当日", "宿泊料金の100%"],
            ["宿都合・決済失敗・期限切れ", "無料"],
          ].map(([when, rate]) => (
            <tr key={when}>
              <td>{when}</td>
              <td className="text-right">{rate}</td>
            </tr>
          ))}
        </tbody>
      </table>

      {booking && cancellable && (
        <Button variant="secondary" className="h-11" onClick={() => setConfirming(true)}>
          この予約をキャンセルする
        </Button>
      )}

      {confirming && booking && (
        <Modal title="この予約をキャンセルしますか" onClose={() => setConfirming(false)} width={520}>
          <div className="dialog-body">
            <p className="mb-[14px]">
              {fmt(booking.checkinDate)} からのご予約をキャンセルします。キャンセル料は規定に従って確定します。この操作は取り消せません。
            </p>
          </div>
          <div className="dialog-actions">
            <Button variant="secondary" onClick={() => setConfirming(false)}>
              やめる
            </Button>
            <Button variant="danger" onClick={doCancel}>
              キャンセルする
            </Button>
          </div>
        </Modal>
      )}
    </div>
  );
}

export function StatusTag({ status }: { status: BookingStatus }) {
  const cls: Record<BookingStatus, string> = {
    temporary_hold: "tag-accent-2",
    processing_payment: "tag-outline",
    confirmed: "tag-accent",
    cancelled: "tag-neutral",
  };
  return <span className={`tag ${cls[status]}`}>{statusLabel[status]}</span>;
}
