"use client";

import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react";
import { createJudgeClient, probeCookieSession, type JudgeClient } from "@/lib/api";
import {
  actorOf,
  devHeadersOf,
  identityOf,
  SEED_USERS,
  type Actor,
  type DevUser,
  type SessionState,
} from "@/lib/session";
import type { Me } from "@/lib/types";

export interface AuthValue {
  client: JudgeClient;
  state: SessionState;
  actor: Actor | null;
  identity: string;
  /**
   * The identity the first session check found: whom the server rendered the
   * page for. Null until that check is done. See canUseServerData.
   */
  serverIdentity: string | null;
  logout: () => Promise<void>;
  /** Reload the signed-in user's /me, for example after creating the handle. */
  refresh: () => Promise<void>;
  setDevEnabled: (value: boolean) => void;
  setDevUser: (value: DevUser) => void;
  switchDevUser: () => void;
}

const AuthContext = createContext<AuthValue | null>(null);

export function useAuth(): AuthValue {
  const value = useContext(AuthContext);
  if (!value) {
    throw new Error("useAuth must be used inside AuthProvider");
  }
  return value;
}

/** Owns the session for every page. Mounted once in the root layout. */
export function AuthProvider({ children }: { children: ReactNode }) {
  const [state, setState] = useState<SessionState>({
    session: null,
    cookieChecked: false,
    devEnabled: false,
    devUser: { userId: SEED_USERS[0].userId, role: SEED_USERS[0].role },
    devMe: null,
  });

  const [serverIdentity, setServerIdentity] = useState<string | null>(null);
  const devHeaders = useMemo(() => devHeadersOf(state), [state]);
  const client = useMemo(() => createJudgeClient(() => devHeaders), [devHeaders]);

  const commitSession = useCallback((session: Me | null) => {
    setState((current) => ({
      ...current,
      session,
      cookieChecked: true,
      // A real cookie session always wins: force dev auth off.
      devEnabled: session ? false : current.devEnabled,
    }));
    // The first check tells whom the server rendered for (it sees cookies
    // only); later commits (logout, refresh) never move it.
    const rendered = identityOf({ session, cookieChecked: true, devEnabled: false, devUser: SEED_USERS[0], devMe: null });
    setServerIdentity((first) => first ?? rendered);
  }, []);

  const fetchSession = useCallback(async (): Promise<Me | null> => {
    try {
      return await probeCookieSession();
    } catch {
      return null;
    }
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

  // A dev user's handle comes from /me asked with their dev headers.
  const fetchDevMe = useCallback(async (): Promise<Me | null> => {
    if (!devHeaders) {
      return null;
    }
    try {
      return await client.getMe();
    } catch {
      return null;
    }
  }, [client, devHeaders]);

  const commitDevMe = useCallback((devMe: Me | null) => {
    setState((current) => ({ ...current, devMe }));
  }, []);

  useEffect(() => {
    let cancelled = false;
    // Commit state only in the promise callback (never synchronously in the effect).
    void fetchDevMe().then((devMe) => {
      if (!cancelled) {
        commitDevMe(devMe);
      }
    });
    return () => {
      cancelled = true;
    };
  }, [fetchDevMe, commitDevMe]);

  const logout = useCallback(async () => {
    await client.logout();
    commitSession(await fetchSession());
  }, [client, fetchSession, commitSession]);

  const refresh = useCallback(async () => {
    if (devHeaders) {
      commitDevMe(await fetchDevMe());
      return;
    }
    commitSession(await fetchSession());
  }, [devHeaders, fetchDevMe, commitDevMe, fetchSession, commitSession]);

  const setDevEnabled = useCallback((value: boolean) => {
    setState((current) => ({ ...current, devEnabled: value }));
  }, []);

  const setDevUser = useCallback((value: DevUser) => {
    setState((current) => ({ ...current, devUser: value }));
  }, []);

  const switchDevUser = useCallback(() => {
    setState((current) => {
      const next = SEED_USERS.find((user) => user.userId !== current.devUser.userId) ?? SEED_USERS[0];
      return { ...current, devUser: { userId: next.userId, role: next.role } };
    });
  }, []);

  const value = useMemo<AuthValue>(
    () => ({
      client,
      state,
      actor: actorOf(state),
      identity: identityOf(state),
      serverIdentity,
      logout,
      refresh,
      setDevEnabled,
      setDevUser,
      switchDevUser,
    }),
    [client, state, serverIdentity, logout, refresh, setDevEnabled, setDevUser, switchDevUser],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

/**
 * Remounts the page whenever the effective actor changes, so data loaded for
 * the previous actor is dropped and its in-flight requests are aborted.
 *
 * It renders at once, even before the cookie session is known, so server-
 * rendered content shows on the first paint; client fetches wait for the
 * session check inside useResource instead.
 */
export function ActorScope({ children }: { children: ReactNode }) {
  const { identity } = useAuth();
  return <div key={identity}>{children}</div>;
}
