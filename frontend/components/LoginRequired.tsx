"use client";

import { GOOGLE_LOGIN_PATH } from "@/lib/api";

export function LoginRequired({ title }: { title: string }) {
  return (
    <div className="page">
      <h1>{title}</h1>
      <p className="empty">
        ログインすると表示されます。{" "}
        <a href={GOOGLE_LOGIN_PATH}>Google でログイン</a>
      </p>
    </div>
  );
}
