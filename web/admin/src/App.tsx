import { useEffect, useState } from "react";
import { NavLink, Route, Routes, useLocation } from "react-router-dom";
import type { User } from "oidc-client-ts";
import { getUser, logout, userManager } from "./auth";
import RequireLogin from "./components/RequireLogin";
import AccommodationList from "./pages/AccommodationList";
import AccommodationNew from "./pages/AccommodationNew";
import AccommodationDetail from "./pages/AccommodationDetail";
import InventoryCalendar from "./pages/InventoryCalendar";
import Callback from "./pages/Callback";

const navClass = ({ isActive }: { isActive: boolean }) =>
  `block w-full rounded-[var(--radius-md)] px-3 py-2 text-left text-[14px] ${
    isActive
      ? "bg-[var(--color-accent-100)] text-[var(--color-accent-800)]"
      : "text-[var(--color-text)] hover:bg-[color-mix(in_srgb,var(--color-text)_6%,transparent)]"
  }`;

export default function App() {
  const location = useLocation();
  if (location.pathname === "/callback") return <Callback />;

  return (
    <RequireLogin>
      <Shell />
    </RequireLogin>
  );
}

function Shell() {
  const [user, setUser] = useState<User | null>(null);

  useEffect(() => {
    getUser().then(setUser);
    const onLoaded = (u: User) => setUser(u);
    userManager.events.addUserLoaded(onLoaded);
    return () => userManager.events.removeUserLoaded(onLoaded);
  }, []);

  const name = user?.profile.name ?? user?.profile.email ?? "";

  return (
    <div className="flex min-h-screen">
      <aside className="sticky top-0 flex h-screen w-[218px] shrink-0 flex-col gap-[26px] px-[18px] py-[22px]">
        <div>
          <div className="text-[22px] font-semibold tracking-[-0.02em]" style={{ fontFamily: "var(--font-heading)" }}>
            yadori
          </div>
          <div className="kicker mt-0.5 text-[var(--color-neutral-600)]">運営管理</div>
        </div>
        <nav className="flex flex-col gap-0.5">
          <div className="mb-1.5 text-[10px] uppercase tracking-[0.12em] text-[var(--color-neutral-500)]">
            Menu
          </div>
          <NavLink to="/" end className={navClass}>
            宿一覧
          </NavLink>
        </nav>
        <div className="mt-auto text-[12px] leading-[1.7] text-[var(--color-neutral-600)]">
          <div className="text-[var(--color-text)]">{name}</div>
          <button className="btn btn-ghost -ml-[5px] mt-1.5 text-[12px]" onClick={() => logout()}>
            ログアウト
          </button>
        </div>
      </aside>

      <main className="flex min-w-0 flex-1 flex-col gap-[22px] px-[34px] pt-[26px] pb-[60px]">
        <Routes>
          <Route path="/" element={<AccommodationList />} />
          <Route path="/accommodations/new" element={<AccommodationNew />} />
          <Route path="/accommodations/:accommodationId" element={<AccommodationDetail />} />
          <Route path="/room-types/:roomTypeId/inventories" element={<InventoryCalendar />} />
        </Routes>
      </main>
    </div>
  );
}
