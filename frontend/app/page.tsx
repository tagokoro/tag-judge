"use client";

import { useCallback, useEffect, useMemo, useReducer } from "react";
import { createJudgeClient, probeCookieSession } from "@/lib/api";
import {
  effectiveVersion,
  initialWorkspaceState,
  workspaceReducer,
  type DataGuard,
  type ScopedData,
} from "@/lib/workspaceState";
import type { DevHeaders, Me, Role } from "@/lib/types";
import { WorkspaceContext, type WorkspaceValue } from "@/components/workspace";
import { SessionBar } from "@/components/SessionBar";
import { NamespacePanel } from "@/components/NamespacePanel";
import { LibraryPanel } from "@/components/LibraryPanel";
import { VersionPanel } from "@/components/VersionPanel";
import { SubmissionUrlPanel } from "@/components/SubmissionUrlPanel";

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

  // The DataGuard is captured from the CURRENT state on each render. An async
  // action (run(async ...)) closes over the helpers/guard that were current when
  // it STARTED, so a result that arrives after an actor OR a selection change is
  // dropped by the reducer. It carries both generations: authGeneration (actor)
  // and selectionGeneration (target / failed 404).
  const guard = useMemo<DataGuard>(
    () => ({
      authGeneration: state.authGeneration,
      selectionGeneration: state.selectionGeneration,
    }),
    [state.authGeneration, state.selectionGeneration],
  );

  // The selection setters optionally accept a `patch`. A flow that both CHANGES
  // the target (clicking a library/version) and loads data for the NEW target
  // must do so ATOMICALLY: the reducer clears the stale data, bumps the
  // selection generation, and applies the patch in one dispatch, so the fresh
  // data is never wiped by the clear and a stale (guarded) fetch can never
  // override a newer selection. Pure navigation (typing a slug) passes no patch
  // and no guard: it is a synchronous user intent and is always applied.
  const setNamespaceSlug = useCallback(
    (slug: string, patch?: Partial<ScopedData>) =>
      dispatch(
        patch === undefined
          ? { type: "SET_NAMESPACE_SLUG", slug }
          : { type: "SET_NAMESPACE_SLUG", slug, patch, guard },
      ),
    [dispatch, guard],
  );
  const setLibrarySlug = useCallback(
    (slug: string, patch?: Partial<ScopedData>) =>
      dispatch(
        patch === undefined
          ? { type: "SET_LIBRARY_SLUG", slug }
          : { type: "SET_LIBRARY_SLUG", slug, patch, guard },
      ),
    [dispatch, guard],
  );
  const setVersion = useCallback(
    (version: number | null, patch?: Partial<ScopedData>) =>
      dispatch(
        patch === undefined
          ? { type: "SET_VERSION", version }
          : { type: "SET_VERSION", version, patch, guard },
      ),
    [dispatch, guard],
  );

  // Guarded by the CURRENT authGeneration AND selectionGeneration: a fetch
  // started under a previous actor OR a previous target is dropped by the
  // reducer instead of restoring private/foreign data.
  const patchData = useCallback(
    (patch: Partial<ScopedData>) => dispatch({ type: "APPLY_DATA", patch, guard }),
    [dispatch, guard],
  );

  const clearLibraryDetail = useCallback(
    () => dispatch({ type: "CLEAR_LIBRARY_DETAIL", guard }),
    [dispatch, guard],
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
            認証・利用者の切替があったときは、表示中の Namespace / Library 詳細・Version ソース・提出 URL をすべてクリアし、選択 Version をリセットし、実行中の取得を無効化します。
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
          <SubmissionUrlPanel />
        </div>

        <footer className="app-footer">
          <p className="muted">
            API 契約: <code>backend/docs/openapi.yaml</code> ／ 要件: <code>docs/library-sharing-judge-requirements.md</code>
          </p>
          <p className="muted">
            Library / Version のソースと提出 URL はすべてテキストとして表示し、HTML として解釈しません。private Library の提出 URL は所有者以外には API の認可により表示されません。
          </p>
        </footer>
      </main>
    </WorkspaceContext.Provider>
  );
}
