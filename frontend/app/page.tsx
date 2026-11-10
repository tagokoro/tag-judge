"use client";

import { useCallback, useEffect, useMemo, useReducer } from "react";
import { createJudgeClient, probeCookieSession } from "@/lib/api";
import {
  effectiveVersion,
  initialWorkspaceState,
  workspaceReducer,
  type ScopedData,
} from "@/lib/workspaceState";
import type { DevHeaders, Me, Role } from "@/lib/types";
import { WorkspaceContext, type WorkspaceValue } from "@/components/workspace";
import { SessionBar } from "@/components/SessionBar";
import { NamespacePanel } from "@/components/NamespacePanel";
import { LibraryPanel } from "@/components/LibraryPanel";
import { VersionPanel } from "@/components/VersionPanel";
import { VerificationPanel } from "@/components/VerificationPanel";

export default function Home() {
  const [state, dispatch] = useReducer(workspaceReducer, initialWorkspaceState);

  // A real cookie session always wins; dev auth is strictly opt-in via the
  // SessionBar checkbox. Reads reducer state directly (no refs during render).
  const getDevHeaders = useCallback((): DevHeaders | null => {
    if (state.session) {
      return null;
    }
    if (!state.devEnabled) {
      return null;
    }
    return {
      "X-User-ID": state.devUser.userId,
      "X-User-Role": state.devUser.role,
    };
  }, [state.session, state.devEnabled, state.devUser]);

  const client = useMemo(() => createJudgeClient(getDevHeaders), [getDevHeaders]);

  // Pure fetch: resolves with the cookie-session actor, or null on 401.
  const fetchSession = useCallback(async (): Promise<Me | null> => {
    try {
      return await probeCookieSession();
    } catch {
      return null;
    }
  }, []);

  const commitSession = useCallback(
    (me: Me | null) => dispatch({ type: "SESSION_COMMITTED", session: me }),
    [dispatch],
  );

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

  const setDevEnabled = useCallback(
    (value: boolean) => dispatch({ type: "SET_DEV_ENABLED", value }),
    [dispatch],
  );
  const setDevUserId = useCallback(
    (value: string) => dispatch({ type: "SET_DEV_USER_ID", value }),
    [dispatch],
  );
  const setDevRole = useCallback(
    (value: Role) => dispatch({ type: "SET_DEV_ROLE", value }),
    [dispatch],
  );
  const switchDevUser = useCallback(() => dispatch({ type: "SWITCH_DEV_USER" }), [dispatch]);

  const setNamespaceSlug = useCallback(
    (slug: string) => dispatch({ type: "SET_NAMESPACE_SLUG", slug }),
    [dispatch],
  );
  const setLibrarySlug = useCallback(
    (slug: string) => dispatch({ type: "SET_LIBRARY_SLUG", slug }),
    [dispatch],
  );
  const setVersion = useCallback(
    (version: number | null) => dispatch({ type: "SET_VERSION", version }),
    [dispatch],
  );

  // Guarded by the CURRENT authGeneration: a fetch started under a previous
  // actor is dropped by the reducer instead of restoring private data.
  const patchData = useCallback(
    (patch: Partial<ScopedData>) =>
      dispatch({ type: "APPLY_DATA", patch, guard: { authGeneration: state.authGeneration } }),
    [dispatch, state.authGeneration],
  );

  const clearLibraryDetail = useCallback(
    () =>
      dispatch({
        type: "CLEAR_LIBRARY_DETAIL",
        guard: { authGeneration: state.authGeneration },
      }),
    [dispatch, state.authGeneration],
  );

  const version = effectiveVersion(state);

  const value: WorkspaceValue = useMemo(
    () => ({
      client,
      state,
      dispatch,
      effectiveVersion: version,
      setDevEnabled,
      setDevUserId,
      setDevRole,
      switchDevUser,
      refreshSession,
      logout,
      setNamespaceSlug,
      setLibrarySlug,
      setVersion,
      patchData,
      clearLibraryDetail,
    }),
    [
      client,
      state,
      dispatch,
      version,
      setDevEnabled,
      setDevUserId,
      setDevRole,
      switchDevUser,
      refreshSession,
      logout,
      setNamespaceSlug,
      setLibrarySlug,
      setVersion,
      patchData,
      clearLibraryDetail,
    ],
  );

  return (
    <WorkspaceContext.Provider value={value}>
      <main className="container">
        <header className="app-header">
          <h1>Library Sharing Judge</h1>
          <p>
            競技プログラミング用ライブラリを Namespace に置き、不変な Version を作り、関連する提出 URL を自己宣言で添付できる MVP です。
          </p>
          <p className="note">
            すべての API 呼び出しは同一オリジンの <code>/api/v1/*</code> に対して <code>credentials: &quot;same-origin&quot;</code> で行います。Google ログインも同一オリジンのリンクです。dev ヘッダー認証はチェックを入れたときだけ有効になるローカル開発専用の機能です。
          </p>
          <p className="note">
            認証・利用者の切替があったときは、表示中の Namespace / Library 詳細・Version ソース・提出リンクをすべてクリアし、選択 Version をリセットし、実行中の取得を無効化します。
          </p>
        </header>

        {/* Keying by authGeneration remounts every panel on an actor change:
            local UI state and in-flight fetches (AbortController cleanup) are
            reset, and the reducer has already cleared state.data. */}
        <SessionBar key={state.authGeneration} />

        <div className="grid" key={state.authGeneration}>
          <NamespacePanel />
          <LibraryPanel />
          <VersionPanel />
          <VerificationPanel />
        </div>

        <footer className="app-footer">
          <p className="muted">
            API 契約: <code>backend/docs/openapi.yaml</code> ／ 設計: <code>docs/superpowers/specs/2026-11-10-verification-url-design.md</code>
          </p>
          <p className="muted">
            Library / Version のソースと提出 URL はすべてテキストとして表示し、HTML として解釈しません。private Library のリンクは所有者以外には API の認可により表示されません。
          </p>
        </footer>
      </main>
    </WorkspaceContext.Provider>
  );
}
