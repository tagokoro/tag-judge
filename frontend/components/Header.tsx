"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { GOOGLE_LOGIN_PATH } from "@/lib/api";
import { HOME_PATH, ME_PATH } from "@/lib/routes";
import type { Role } from "@/lib/types";
import { useAuth } from "./AuthProvider";
import { useAsyncAction } from "./useAsyncAction";

const SHOW_DEV_AUTH = process.env.NODE_ENV !== "production";

export function Header() {
  const { state, actor, logout } = useAuth();
  const { busy, error, run } = useAsyncAction();
  const pathname = usePathname();

  return (
    <header className="site-header">
      <div className="site-header-inner">
        <Link href={HOME_PATH} className="brand">
          Library Sharing Judge
        </Link>
        <nav className="site-nav">
          <Link href={ME_PATH} className={pathname === ME_PATH ? "active" : undefined}>
            自分の Namespace
          </Link>
        </nav>
        <div className="site-actions">
          {SHOW_DEV_AUTH && !state.session ? <DevAuthMenu /> : null}
          {!state.cookieChecked ? null : actor ? (
            <>
              <span className="pill" title={actor.userId}>
                {actor.source === "dev" ? "dev: " : ""}
                {actor.role} / {actor.userId.slice(0, 8)}
              </span>
              {actor.source === "cookie" ? (
                <button
                  type="button"
                  className="btn btn-ghost"
                  onClick={() => run(() => logout())}
                  disabled={busy}
                >
                  ログアウト
                </button>
              ) : null}
            </>
          ) : (
            // A plain same-origin link (not fetch): the browser follows the 302 to Google.
            <a className="btn btn-primary" href={GOOGLE_LOGIN_PATH}>
              Google でログイン
            </a>
          )}
        </div>
      </div>
      {error ? (
        <p className="error site-header-error" role="alert">
          {error}
        </p>
      ) : null}
    </header>
  );
}

/** Local-development only: opt-in X-User-ID / X-User-Role headers. */
function DevAuthMenu() {
  const { state, setDevEnabled, setDevUser, switchDevUser } = useAuth();
  const { devEnabled, devUser } = state;

  return (
    <details className="dev-menu">
      <summary className="btn btn-ghost">dev 認証{devEnabled ? "（有効）" : ""}</summary>
      <div className="dev-menu-panel stack">
        <div className="warn" role="note">
          <code>DEV_HEADER_AUTH=true</code> のローカル環境専用です。本番では使わないでください。有効にしたときだけ{" "}
          <code>X-User-ID</code> / <code>X-User-Role</code> を送ります。
        </div>
        <label className="checkbox">
          <input
            type="checkbox"
            checked={devEnabled}
            onChange={(event) => setDevEnabled(event.target.checked)}
          />
          dev ヘッダー認証を使う
        </label>
        {devEnabled ? (
          <>
            <label className="field">
              <span className="field-label">ユーザー ID</span>
              <input
                value={devUser.userId}
                onChange={(event) => setDevUser({ ...devUser, userId: event.target.value })}
                spellCheck={false}
              />
            </label>
            <label className="field">
              <span className="field-label">ロール</span>
              <select
                value={devUser.role}
                onChange={(event) => setDevUser({ ...devUser, role: event.target.value as Role })}
              >
                <option value="author">author</option>
                <option value="admin">admin</option>
                <option value="user">user</option>
              </select>
            </label>
            <button type="button" className="btn" onClick={switchDevUser}>
              seed ユーザーを切替
            </button>
          </>
        ) : null}
      </div>
    </details>
  );
}
