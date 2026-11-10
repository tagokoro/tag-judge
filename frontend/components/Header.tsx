"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useEffect, useRef, useState } from "react";
import { GOOGLE_LOGIN_PATH } from "@/lib/api";
import {
  HOME_PATH,
  ME_PATH,
  MY_PROBLEMS_PATH,
  MY_SUBMISSIONS_PATH,
  PROBLEMS_PATH,
  SUBMISSIONS_PATH,
} from "@/lib/routes";
import type { Role } from "@/lib/types";
import { useAuth } from "./AuthProvider";
import { useAsyncAction } from "./useAsyncAction";

const SHOW_DEV_AUTH = process.env.NODE_ENV !== "production";

const NAV_ITEMS = [
  { href: HOME_PATH, label: "ライブラリ一覧" },
  { href: PROBLEMS_PATH, label: "問題一覧" },
  { href: SUBMISSIONS_PATH, label: "提出一覧" },
];

export function Header() {
  const { state, actor } = useAuth();
  const pathname = usePathname();

  return (
    <header className="site-header">
      <div className="site-header-inner">
        <Link href={HOME_PATH} className="brand">
          <VersionMark />
          TagJudge
        </Link>
        <nav className="site-nav" aria-label="メインメニュー">
          {NAV_ITEMS.map((item) => (
            <Link
              key={item.href}
              href={item.href}
              className={pathname === item.href ? "active" : undefined}
              aria-current={pathname === item.href ? "page" : undefined}
            >
              {item.label}
            </Link>
          ))}
        </nav>
        <div className="site-actions">
          {SHOW_DEV_AUTH && !state.session ? <DevAuthMenu /> : null}
          {!state.cookieChecked ? null : actor ? (
            <UserMenu />
          ) : (
            // A plain same-origin link (not fetch): the browser follows the 302 to Google.
            <a className="btn btn-primary" href={GOOGLE_LOGIN_PATH}>
              Google でログイン
            </a>
          )}
        </div>
      </div>
    </header>
  );
}

/** The signed-in user's menu: their own lists and logout. */
function UserMenu() {
  const { actor, logout } = useAuth();
  const { busy, error, run } = useAsyncAction();
  const [open, setOpen] = useState(false);
  const rootRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) {
      return;
    }
    const close = (event: MouseEvent | KeyboardEvent) => {
      if (event instanceof KeyboardEvent ? event.key === "Escape" : !rootRef.current?.contains(event.target as Node)) {
        setOpen(false);
      }
    };
    document.addEventListener("mousedown", close);
    document.addEventListener("keydown", close);
    return () => {
      document.removeEventListener("mousedown", close);
      document.removeEventListener("keydown", close);
    };
  }, [open]);

  if (!actor) {
    return null;
  }
  const items = [
    { href: ME_PATH, label: "自分のライブラリ" },
    { href: MY_SUBMISSIONS_PATH, label: "自分の提出" },
    { href: MY_PROBLEMS_PATH, label: "自分の問題" },
  ];

  return (
    <div className="user-menu" ref={rootRef}>
      <button
        type="button"
        className="user-button"
        aria-haspopup="menu"
        aria-expanded={open}
        onClick={() => setOpen((value) => !value)}
        title={actor.userId}
      >
        <span className="user-avatar" aria-hidden="true">
          {actor.userId.slice(0, 1).toUpperCase()}
        </span>
        <span className="user-name">
          {actor.source === "dev" ? "dev " : ""}
          {actor.userId.slice(0, 8)}
        </span>
        <span className="user-role">{actor.role}</span>
      </button>
      {open ? (
        <div className="user-menu-panel" role="menu">
          {items.map((item) => (
            <Link key={item.href} href={item.href} role="menuitem" onClick={() => setOpen(false)}>
              {item.label}
            </Link>
          ))}
          {actor.source === "cookie" ? (
            <>
              <hr />
              <button
                type="button"
                role="menuitem"
                className="menu-danger"
                onClick={() => run(async () => {
                  await logout();
                  setOpen(false);
                })}
                disabled={busy}
              >
                ログアウト
              </button>
            </>
          ) : (
            <p className="user-menu-note">dev 認証中はログアウトの代わりに dev 認証を無効にしてください。</p>
          )}
          {error ? (
            <p className="error" role="alert">
              {error}
            </p>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}

/** Three versions on one line: the site's version-line motif. */
function VersionMark() {
  return (
    <svg className="brand-mark" width="34" height="14" viewBox="0 0 34 14" aria-hidden="true">
      <line x1="5" y1="7" x2="29" y2="7" stroke="var(--mizu)" strokeWidth="2" />
      <circle cx="5" cy="7" r="4" fill="var(--paper)" stroke="var(--mizu)" strokeWidth="2" />
      <circle cx="17" cy="7" r="4" fill="var(--paper)" stroke="var(--mizu)" strokeWidth="2" />
      <circle cx="29" cy="7" r="5" fill="var(--deep)" />
    </svg>
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
