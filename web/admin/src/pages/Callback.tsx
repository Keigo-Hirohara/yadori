import { useEffect, useState } from "react";
import { useNavigate } from "react-router-dom";
import { completeLogin } from "../auth";
import { Empty, ErrorBanner } from "../components/ui";

export default function Callback() {
  const navigate = useNavigate();
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    completeLogin()
      .then((returnTo) => navigate(returnTo, { replace: true }))
      .catch((e) => setError((e as Error).message));
  }, [navigate]);

  return (
    <div className="mx-auto max-w-[860px] px-6 pt-12">
      <ErrorBanner message={error} />
      {!error && <Empty>ログインしています…</Empty>}
    </div>
  );
}
