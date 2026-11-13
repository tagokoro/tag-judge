"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import type { JudgeClient } from "@/lib/api";
import { canUseServerData } from "@/lib/session";
import { useAuth } from "./AuthProvider";

interface Loaded<T> {
  key: string;
  data?: T;
  error?: unknown;
}

export interface Resource<T> {
  data: T | null;
  error: unknown;
  loading: boolean;
  reload: () => void;
  /** Replace the data after a successful write, without refetching. */
  setData: (data: T) => void;
}

/**
 * Loads one API resource for the current actor. `key` names what is loaded
 * (pass null to skip). A result is shown only for the key and actor it was
 * requested for, and a superseded request is aborted, so a late response can
 * never show data for another target or another actor.
 *
 * Nothing is requested until the cookie session is known, so a page never
 * fetches as anonymous and then again as the signed-in user.
 *
 * `initial` is data a Server Component already fetched with the visitor's
 * cookies. It is used as is, without a request, only while the actor is the
 * one the server rendered for: not with dev header auth (the server never sees
 * dev headers), not after a logout or login on this page, and not after
 * `reload`. Otherwise it is fetched again for the current actor.
 */
export function useResource<T>(
  key: string | null,
  load: (api: JudgeClient) => Promise<T>,
  initial?: { data: T },
): Resource<T> {
  const { client, identity, serverIdentity, state } = useAuth();
  const [tick, setTick] = useState(0);
  const [loaded, setLoaded] = useState<Loaded<T> | null>(null);

  // The latest loader, synced in an effect so it is never touched during render.
  const loadRef = useRef(load);
  useEffect(() => {
    loadRef.current = load;
  });

  const useInitial = initial !== undefined && tick === 0 && canUseServerData(state, serverIdentity);
  const fullKey =
    key === null || useInitial || !state.cookieChecked ? null : `${identity}\n${key}\n${tick}`;

  useEffect(() => {
    if (fullKey === null) {
      return;
    }
    const controller = new AbortController();
    loadRef.current(client.withSignal(controller.signal)).then(
      (data) => {
        if (!controller.signal.aborted) {
          setLoaded({ key: fullKey, data });
        }
      },
      (error: unknown) => {
        if (!controller.signal.aborted) {
          setLoaded({ key: fullKey, error });
        }
      },
    );
    return () => controller.abort();
  }, [fullKey, client]);

  const reload = useCallback(() => setTick((value) => value + 1), []);
  const setData = useCallback(
    (data: T) => {
      if (fullKey !== null) {
        setLoaded({ key: fullKey, data });
      }
    },
    [fullKey],
  );

  if (useInitial) {
    return { data: initial.data, error: null, loading: false, reload, setData };
  }
  const current = loaded !== null && loaded.key === fullKey ? loaded : null;
  return {
    data: current?.data ?? null,
    error: current?.error ?? null,
    // Still waiting for the session check counts as loading.
    loading: key !== null && current === null,
    reload,
    setData,
  };
}
