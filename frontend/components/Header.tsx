"use client";

import Image from "next/image";
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
import { DEV_AUTH_AVAILABLE, type Actor } from "@/lib/session";
import type { Role } from "@/lib/types";
import logo from "@/assets/tagjudge-logo.png";
import { useAuth } from "./AuthProvider";
import { Avatar } from "./ui";
import { useAsyncAction } from "./useAsyncAction";


const NAV_ITEMS = [
  { href: HOME_PATH, label: "Libraries" },
  { href: PROBLEMS_PATH, label: "Problems" },
  { href: SUBMISSIONS_PATH, label: "Submissions" },
];

export function Header() {
  const { state, actor } = useAuth();
  const pathname = usePathname();
  const [sheetOpen, setSheetOpen] = useState(false);

  // A new page closes the phone menu.
  const [sheetPath, setSheetPath] = useState(pathname);
  if (sheetPath !== pathname) {
    setSheetPath(pathname);
    setSheetOpen(false);
  }

  useEffect(() => {
    if (!sheetOpen) {
      return;
    }
    const close = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        setSheetOpen(false);
      }
    };
    document.addEventListener("keydown", close);
    return () => document.removeEventListener("keydown", close);
  }, [sheetOpen]);

  return (
    <header className="site-header">
      <div className="site-header-inner">
        <Link href={HOME_PATH} className="brand">
          {/* On every page above the fold, so it loads at once instead of lazily. */}
          <Image src={logo} alt="TagJudge" className="brand-logo" loading="eager" />
        </Link>
        <nav className="site-nav" aria-label="メインメニュー">
          <NavLinks pathname={pathname} />
        </nav>
        <div className="site-actions">
          {DEV_AUTH_AVAILABLE && !state.session ? <DevAuthMenu /> : null}
          {!state.cookieChecked ? null : actor ? (
            <UserMenu />
          ) : (
            // A plain same-origin link (not fetch): the browser follows the 302 to Google.
            <a className="btn btn-primary" href={GOOGLE_LOGIN_PATH}>
              Google でログイン
            </a>
          )}
        </div>
        <button
          type="button"
          className="menu-toggle"
          aria-expanded={sheetOpen}
          aria-controls="site-sheet"
          aria-label={sheetOpen ? "メニューを閉じる" : "メニューを開く"}
          onClick={() => setSheetOpen((value) => !value)}
        >
          <MenuIcon open={sheetOpen} />
        </button>
      </div>
      {sheetOpen ? <PhoneSheet pathname={pathname} /> : null}
    </header>
  );
}

function NavLinks({ pathname }: { pathname: string }) {
  return NAV_ITEMS.map((item) => (
    <Link
      key={item.href}
      href={item.href}
      className={pathname === item.href ? "active" : undefined}
      aria-current={pathname === item.href ? "page" : undefined}
    >
      {item.label}
    </Link>
  ));
}

const MY_ITEMS = [
  { href: ME_PATH, label: "自分のライブラリ" },
  { href: MY_SUBMISSIONS_PATH, label: "自分の提出" },
  { href: MY_PROBLEMS_PATH, label: "自分の問題" },
];

/** The phone menu: every page link and the account actions in one sheet. */
function PhoneSheet({ pathname }: { pathname: string }) {
  const { state, actor, logout } = useAuth();
  const { busy, error, run } = useAsyncAction();

  return (
    <div id="site-sheet" className="site-sheet">
      <nav className="sheet-group" aria-label="メインメニュー（スマートフォン）">
        <NavLinks pathname={pathname} />
      </nav>
      {!state.cookieChecked ? null : actor ? (
        <div className="sheet-group">
          <p className="sheet-who">
            {whoOf(actor)} <span className="user-role">{actor.role}</span>
          </p>
          {MY_ITEMS.map((item) => (
            <Link
              key={item.href}
              href={item.href}
              aria-current={pathname === item.href ? "page" : undefined}
            >
              {item.label}
            </Link>
          ))}
          {actor.source === "cookie" ? (
            <button type="button" className="menu-danger" onClick={() => run(logout)} disabled={busy}>
              ログアウト
            </button>
          ) : null}
          {error ? (
            <p className="error" role="alert">
              {error}
            </p>
          ) : null}
        </div>
      ) : (
        <div className="sheet-group">
          <a className="btn btn-primary" href={GOOGLE_LOGIN_PATH}>
            Google でログイン
          </a>
        </div>
      )}
      {DEV_AUTH_AVAILABLE && !state.session ? (
        <div className="sheet-group">
          <DevAuthMenu />
        </div>
      ) : null}
    </div>
  );
}

function MenuIcon({ open }: { open: boolean }) {
  return (
    <svg width="22" height="22" viewBox="0 0 22 22" aria-hidden="true">
      {open ? (
        <path d="M5 5l12 12M17 5L5 17" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
      ) : (
        <path d="M3 6h16M3 11h16M3 16h16" stroke="currentColor" strokeWidth="2" strokeLinecap="round" />
      )}
    </svg>
  );
}

/** How the account is named in the menus: the handle, marked when it is a dev-header actor. */
function whoOf(actor: Actor): string {
  return `${actor.source === "dev" ? "dev " : ""}${actor.handle ? `@${actor.handle}` : "ハンドル未登録"}`;
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
  // The button shows only the avatar, so the name lives in its label and atop the menu.
  const who = whoOf(actor);

  return (
    <div className="user-menu" ref={rootRef}>
      <button
        type="button"
        className="user-button"
        aria-haspopup="menu"
        aria-expanded={open}
        aria-label={`アカウントメニュー（${who}）`}
        title={who}
        onClick={() => setOpen((value) => !value)}
      >
        <Avatar name={actor.handle ?? "?"} />
      </button>
      {open ? (
        <div className="user-menu-panel" role="menu">
          <p className="user-menu-who">
            <span className="user-menu-handle">{who}</span>
            <span className="user-role">{actor.role}</span>
          </p>
          {MY_ITEMS.map((item) => (
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
