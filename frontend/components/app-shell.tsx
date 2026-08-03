"use client";

import Link from "next/link";
import { LogIn, LogOut, Settings, ShieldCheck } from "lucide-react";
import { useEffect, useState, useSyncExternalStore } from "react";
import { api, ApiError } from "@/lib/api";
import { defaultDevAuth, readDevAuth, subscribeDevAuth, writeDevAuth } from "@/lib/auth-store";
import type { DevAuth, Me, Role } from "@/lib/types";

export function AppShell({ children }: { children: React.ReactNode }) {
  const auth = useSyncExternalStore(subscribeDevAuth, readDevAuth, () => null);
  const [me, setMe] = useState<Me | null>(null);
  const [authOpen, setAuthOpen] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    let ignore = false;
    api
      .me()
      .then((value) => {
        if (!ignore) {
          setMe(value);
          setError(null);
        }
      })
      .catch((err: unknown) => {
        if (!ignore) {
          setMe(null);
          setError(err instanceof ApiError && err.status !== 401 ? err.message : null);
        }
      });
    return () => {
      ignore = true;
    };
  }, [auth]);

  async function logout() {
    await api.logout().catch(() => undefined);
    writeDevAuth(null);
    setMe(null);
  }

  return (
    <div className="min-h-screen">
      <header className="border-b border-line bg-white">
        <div className="mx-auto flex max-w-7xl flex-wrap items-center justify-between gap-3 px-4 py-3 sm:px-6">
          <Link href="/problems" className="text-lg font-bold tracking-normal text-ink">
            Tag Judge
          </Link>
          <nav className="flex items-center gap-2">
            <Link href="/problems" className="button-secondary">
              Problems
            </Link>
            <Link href="/problems/new" className="button-primary">
              New
            </Link>
            <button className="button-secondary" type="button" onClick={() => setAuthOpen((value) => !value)} title="Auth">
              <Settings size={17} />
              <span>{me ? me.role : "Auth"}</span>
            </button>
            {me ? (
              <button className="button-secondary" type="button" onClick={logout} title="Logout">
                <LogOut size={17} />
              </button>
            ) : (
              <a className="button-secondary" href="/api/backend/auth/google/login" title="Google login">
                <LogIn size={17} />
              </a>
            )}
          </nav>
        </div>
        {authOpen ? <AuthPanel auth={auth} me={me} error={error} /> : null}
      </header>
      <main className="mx-auto max-w-7xl px-4 py-6 sm:px-6">{children}</main>
    </div>
  );
}

function AuthPanel({ auth, me, error }: { auth: DevAuth | null; me: Me | null; error: string | null }) {
  const [userId, setUserId] = useState(auth?.userId ?? defaultDevAuth.userId);
  const [role, setRole] = useState<Role>(auth?.role ?? defaultDevAuth.role);

  useEffect(() => {
    setUserId(auth?.userId ?? defaultDevAuth.userId);
    setRole(auth?.role ?? defaultDevAuth.role);
  }, [auth]);

  return (
    <div className="border-t border-line bg-panel">
      <form
        className="mx-auto grid max-w-7xl gap-3 px-4 py-3 sm:grid-cols-[minmax(260px,1fr)_160px_auto_auto] sm:items-end sm:px-6"
        onSubmit={(event) => {
          event.preventDefault();
          writeDevAuth({ userId, role });
        }}
      >
        <label className="field">
          <span className="label">Dev user id</span>
          <input className="input" value={userId} onChange={(event) => setUserId(event.target.value)} />
        </label>
        <label className="field">
          <span className="label">Role</span>
          <select className="input" value={role} onChange={(event) => setRole(event.target.value as Role)}>
            <option value="user">user</option>
            <option value="author">author</option>
            <option value="admin">admin</option>
          </select>
        </label>
        <button className="button-primary" type="submit">
          <ShieldCheck size={17} />
          Apply
        </button>
        <button className="button-secondary" type="button" onClick={() => writeDevAuth(defaultDevAuth)}>
          Seed
        </button>
        <div className="text-sm text-ink/65 sm:col-span-4">
          {me ? `Signed in as ${me.user_id} (${me.role})` : "No active session"}
          {error ? ` / ${error}` : ""}
        </div>
      </form>
    </div>
  );
}
