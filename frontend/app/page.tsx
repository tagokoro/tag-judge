"use client";

import { useCallback, useEffect, useMemo, useState } from "react";
import { createJudgeClient, probeCookieSession } from "@/lib/api";
import {
  DEFAULT_LIBRARY_SLUG,
  DEFAULT_NAMESPACE_SLUG,
  SEED_USERS,
} from "@/lib/verification";
import type { DevHeaders, Me, Role } from "@/lib/types";
import { WorkspaceContext, type WorkspaceValue } from "@/components/workspace";
import { SessionBar } from "@/components/SessionBar";
import { NamespacePanel } from "@/components/NamespacePanel";
import { LibraryPanel } from "@/components/LibraryPanel";
import { VersionPanel } from "@/components/VersionPanel";
import { VerificationPanel } from "@/components/VerificationPanel";

interface DevUser {
  userId: string;
  role: Role;
}

export default function Home() {
  const [session, setSession] = useState<Me | null>(null);
  const [cookieChecked, setCookieChecked] = useState(false);
  const [devEnabled, setDevEnabledState] = useState(false);
  const [devUser, setDevUser] = useState<DevUser>({
    userId: SEED_USERS[0].userId,
    role: SEED_USERS[0].role,
  });

  const [namespaceSlugState, setNamespaceSlugState] = useState(DEFAULT_NAMESPACE_SLUG);
  const [librarySlugState, setLibrarySlugState] = useState(DEFAULT_LIBRARY_SLUG);
  const [versionNumberState, setVersionNumberState] = useState<number | null>(null);
  const [refreshTick, setRefreshTick] = useState(0);

  // A real cookie session always wins; dev auth is strictly opt-in via the
  // SessionBar checkbox. This reads state directly (no refs during render).
  const getDevHeaders = useCallback((): DevHeaders | null => {
    if (session) {
      return null;
    }
    if (!devEnabled) {
      return null;
    }
    return {
      "X-User-ID": devUser.userId,
      "X-User-Role": devUser.role,
    };
  }, [session, devEnabled, devUser]);

  const client = useMemo(() => createJudgeClient(getDevHeaders), [getDevHeaders]);

  // Pure fetch: resolves with the cookie-session actor, or null on 401.
  // It contains no setState, so it is safe to start from an effect.
  const fetchSession = useCallback(async (): Promise<Me | null> => {
    try {
      return await probeCookieSession();
    } catch {
      return null;
    }
  }, []);

  const commitSession = useCallback((me: Me | null) => {
    setSession(me);
    // A real cookie session always wins: force dev auth off.
    if (me) {
      setDevEnabledState(false);
    }
    setCookieChecked(true);
  }, []);

  useEffect(() => {
    let cancelled = false;
    // Commit state only in the promise callback (never synchronously in the effect).
    void fetchSession().then((me) => {
      if (!cancelled) {
        commitSession(me);
      }
    });
    return () => {
      cancelled = true;
    };
  }, [fetchSession, commitSession]);

  const refreshSession = useCallback(async () => {
    commitSession(await fetchSession());
  }, [fetchSession, commitSession]);

  const logout = useCallback(async () => {
    await client.logout();
    commitSession(await fetchSession());
  }, [client, fetchSession, commitSession]);

  const setDevEnabled = useCallback((value: boolean) => {
    setDevEnabledState(value);
  }, []);
  const setDevUserId = useCallback((value: string) => {
    setDevUser((previous) => ({ ...previous, userId: value }));
  }, []);
  const setDevRole = useCallback((value: Role) => {
    setDevUser((previous) => ({ ...previous, role: value }));
  }, []);
  const switchDevUser = useCallback(() => {
    setDevUser((previous) =>
      previous.userId === SEED_USERS[0].userId
        ? { userId: SEED_USERS[1].userId, role: SEED_USERS[1].role }
        : { userId: SEED_USERS[0].userId, role: SEED_USERS[0].role },
    );
    // Re-run the current library detail fetch so private->404 / public->200 is
    // observable on the same page (MVP §11.6).
    setRefreshTick((tick) => tick + 1);
  }, []);

  const setNamespaceSlug = useCallback((value: string) => setNamespaceSlugState(value), []);
  const setLibrarySlug = useCallback((value: string) => setLibrarySlugState(value), []);
  const setVersionNumber = useCallback(
    (value: number | null) => setVersionNumberState(value),
    [],
  );

  const value: WorkspaceValue = useMemo(
    () => ({
      client,
      session,
      cookieChecked,
      devEnabled,
      devUserId: devUser.userId,
      devRole: devUser.role,
      setDevEnabled,
      setDevUserId,
      setDevRole,
      switchDevUser,
      refreshSession,
      logout,
      namespaceSlug: namespaceSlugState,
      setNamespaceSlug,
      librarySlug: librarySlugState,
      setLibrarySlug,
      versionNumber: versionNumberState,
      setVersionNumber,
      refreshTick,
    }),
    [
      client,
      session,
      cookieChecked,
      devEnabled,
      devUser,
      setDevEnabled,
      setDevUserId,
      setDevRole,
      switchDevUser,
      refreshSession,
      logout,
      namespaceSlugState,
      setNamespaceSlug,
      librarySlugState,
      setLibrarySlug,
      versionNumberState,
      setVersionNumber,
      refreshTick,
    ],
  );

  return (
    <WorkspaceContext.Provider value={value}>
      <main className="container">
        <header className="app-header">
          <h1>Library Sharing Judge</h1>
          <p>
            競技プログラミング用ライブラリを Namespace に置き、不変な Version を作り、既存 Problem を選んで adapter で検証する MVP です。
          </p>
          <p className="note">
            すべての API 呼び出しは同一オリジンの <code>/api/v1/*</code> に対して <code>credentials: &quot;same-origin&quot;</code> で行います。Google ログインも同一オリジンのリンクです。dev ヘッダー認証はチェックを入れたときだけ有効になるローカル開発専用の機能です。
          </p>
        </header>

        <SessionBar />

        <div className="grid">
          <NamespacePanel />
          <LibraryPanel />
          <VersionPanel />
          <VerificationPanel />
        </div>

        <footer className="app-footer">
          <p className="muted">
            API 契約: <code>backend/docs/openapi.yaml</code> ／ 設計: <code>docs/superpowers/specs/2026-11-08-library-sharing-mvp-design.md</code>
          </p>
          <p className="muted">
            Library / Version のソース、adapter、問題文はすべてテキストとして表示し、HTML として解釈しません。private のソースと他人の adapter は API の認可により表示されません。
          </p>
        </footer>
      </main>
    </WorkspaceContext.Provider>
  );
}
