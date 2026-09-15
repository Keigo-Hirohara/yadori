import { useEffect, useState, type ReactNode } from "react";
import { getUser, login } from "../auth";
import { Button, Empty } from "./ui";

export default function RequireLogin({ children }: { children: ReactNode }) {
  const [state, setState] = useState<"checking" | "in" | "out">("checking");

  useEffect(() => {
    getUser().then((u) => setState(u && !u.expired ? "in" : "out"));
  }, []);

  if (state === "checking") return <Empty>確認しています…</Empty>;
  if (state === "out") {
    return (
      <div className="mx-auto max-w-[560px] px-6 pt-20 text-center">
        <h2 className="mb-8 text-[22px]">ログインが必要です</h2>
        <Button onClick={() => login()}>ログインする</Button>
      </div>
    );
  }
  return <>{children}</>;
}
