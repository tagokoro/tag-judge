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
  logout: () => Promise<void>;
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
  });

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

  const logout = useCallback(async () => {
    await client.logout();
    commitSession(await fetchSession());
  }, [client, fetchSession, commitSession]);

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
      logout,
      setDevEnabled,
      setDevUser,
      switchDevUser,
    }),
    [client, state, logout, setDevEnabled, setDevUser, switchDevUser],
  );

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>;
}

/**
 * Remounts the page whenever the effective actor changes, so data loaded for
 * the previous actor is dropped and its in-flight requests are aborted.
 */
export function ActorScope({ children }: { children: ReactNode }) {
  const { identity, state } = useAuth();
  if (!state.cookieChecked) {
    return <p className="muted">セッションを確認しています…</p>;
  }
  return <div key={identity}>{children}</div>;
}
