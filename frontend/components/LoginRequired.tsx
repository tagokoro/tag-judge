"use client";

import { GOOGLE_LOGIN_PATH } from "@/lib/api";
import { useAuth } from "./AuthProvider";

/** Shown in place of a signed-in page; waits for the session check first. */
export function LoginRequired({ title }: { title: string }) {
  const { state } = useAuth();
  return (
    <div className="page">
      <h1>{title}</h1>
      {state.cookieChecked ? (
        <p className="empty">
          ログインすると表示されます。 <a href={GOOGLE_LOGIN_PATH}>Google でログイン</a>
        </p>
      ) : (
        <p className="muted">読み込み中…</p>
      )}
    </div>
  );
}
