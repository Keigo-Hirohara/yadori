import { useEffect, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { getBooking, pay, type Booking } from "../api/client";
import { Button, ErrorBanner, yen } from "../components/ui";

const HOLD_MINUTES = 30;
type Mode = "success" | "failure" | "timeout";

export default function Payment() {
  const { bookingId = "" } = useParams();
  const navigate = useNavigate();
  const [booking, setBooking] = useState<Booking | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [mode, setMode] = useState<Mode>("success");
  const [outcome, setOutcome] = useState<"failure" | "timeout" | null>(null);
  const [paying, setPaying] = useState(false);
  const [remaining, setRemaining] = useState(HOLD_MINUTES * 60);

  useEffect(() => {
    getBooking(bookingId).then(setBooking).catch((e) => setError(e.message));
  }, [bookingId]);

  useEffect(() => {
    const timer = setInterval(() => setRemaining((r) => Math.max(0, r - 1)), 1000);
    return () => clearInterval(timer);
  }, []);

  const submit = async () => {
    setPaying(true);
    setError(null);
    setOutcome(null);
    try {
      await pay(bookingId, mode);
      navigate(`/bookings/${bookingId}`);
    } catch (e) {
      if (mode === "success") setError((e as Error).message);
      else setOutcome(mode);
    } finally {
      setPaying(false);
    }
  };

  const minutes = String(Math.floor(remaining / 60)).padStart(2, "0");
  const seconds = String(remaining % 60).padStart(2, "0");
  const expired = remaining === 0;

  return (
    <div className="mx-auto max-w-[860px] px-6 pt-12 pb-32">
      <div className="mb-10 flex flex-wrap items-baseline gap-5">
        <h1 className="text-[26px]">お支払い</h1>
        <span className={`text-[13px] ${expired ? "text-[var(--color-accent-2-700)]" : "text-muted"}`}>
          {expired ? "確保の期限が切れました" : `お支払い期限まで ${minutes}:${seconds}`}
        </span>
      </div>
      <ErrorBanner message={error} />

      {booking && (
        <>
          <table className="table mb-6 max-w-[560px]">
            <tbody>
              <tr>
                <td className="text-muted w-[150px]">日程</td>
                <td>
                  {booking.checkinDate} 〜 {booking.checkoutDate}（{booking.nights}泊・{booking.guests.length}名）
                </td>
              </tr>
              <tr>
                <td className="text-muted">宿泊者</td>
                <td>{booking.guests.map((g) => `${g.lastName} ${g.firstName}`).join(" ／ ")}</td>
              </tr>
            </tbody>
          </table>
          <div className="mb-9 flex max-w-[520px] items-baseline justify-between border-t-2 border-[var(--color-text)] pt-4">
            <span className="text-[16px]">お支払い金額</span>
            <strong className="text-[34px]">{yen(booking.totalFee)}</strong>
          </div>
          <Button className="h-[52px] px-10 text-[17px]" onClick={submit} disabled={paying || expired}>
            {paying ? "処理中…" : `${yen(booking.totalFee)} を支払う`}
          </Button>
        </>
      )}

      {outcome === "failure" && (
        <div className="notice notice-accent-2 mt-6 max-w-[560px]">
          <strong>お支払いに失敗しました。</strong>
          <br />
          決済機関から承認が得られませんでした。この予約は取り消されています。
        </div>
      )}
      {outcome === "timeout" && (
        <div className="notice notice-neutral mt-6 max-w-[560px]">
          <strong>決済機関から応答がありません。</strong>
          <br />
          お支払いの結果を確認しています。二重にお支払いいただかないよう、このままお待ちください。予約詳細でも状態をご確認いただけます。
        </div>
      )}

      <div className="devtools mt-14 max-w-[560px]">
        <div className="devtools-label">開発者用 ／ 決済レスポンス切り替え</div>
        <div className="seg">
          {(["success", "failure", "timeout"] as Mode[]).map((m) => (
            <button key={m} type="button" className="seg-opt" aria-pressed={mode === m} onClick={() => setMode(m)}>
              {{ success: "成功", failure: "失敗", timeout: "無応答" }[m]}
            </button>
          ))}
        </div>
        <p className="text-muted mt-2.5 text-[12px]">
          無応答の場合、予約も在庫もそのまま残ります。期限切れ回収が後で決着させます。
        </p>
      </div>
    </div>
  );
}
