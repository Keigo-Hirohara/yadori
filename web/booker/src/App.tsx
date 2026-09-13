import { NavLink, Route, Routes } from "react-router-dom";
import Search from "./pages/Search";
import SearchResults from "./pages/SearchResults";
import BookingNew from "./pages/BookingNew";
import Payment from "./pages/Payment";
import BookingDetail from "./pages/BookingDetail";
import BookingHistory from "./pages/BookingHistory";

const navClass = ({ isActive }: { isActive: boolean }) =>
  `text-[14px] no-underline ${isActive ? "text-[var(--color-accent)]" : "text-inherit hover:text-[var(--color-accent)]"}`;

export default function App() {
  return (
    <div className="min-h-screen">
      <div className="sticky top-0 z-30 bg-[var(--color-bg)]">
        <div className="mx-auto flex max-w-[1080px] items-center gap-8 px-6 py-[18px]">
          <NavLink to="/" className="mr-auto text-[22px] tracking-[0.04em] text-[var(--color-text)] no-underline" style={{ fontFamily: "var(--font-heading)", fontWeight: 600 }}>
            yadori
          </NavLink>
          <NavLink to="/" end className={navClass}>
            検索
          </NavLink>
          <NavLink to="/bookings" className={navClass}>
            履歴
          </NavLink>
        </div>
      </div>

      <Routes>
        <Route path="/" element={<Search />} />
        <Route path="/results" element={<SearchResults />} />
        <Route path="/room-types/:roomTypeId/book" element={<BookingNew />} />
        <Route path="/bookings/:bookingId/payment" element={<Payment />} />
        <Route path="/bookings/:bookingId" element={<BookingDetail />} />
        <Route path="/bookings" element={<BookingHistory />} />
      </Routes>
    </div>
  );
}
