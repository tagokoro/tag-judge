"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import type { JudgeClient } from "@/lib/api";
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
 */
export function useResource<T>(
  key: string | null,
  load: (api: JudgeClient) => Promise<T>,
): Resource<T> {
  const { client, identity } = useAuth();
  const [tick, setTick] = useState(0);
  const [loaded, setLoaded] = useState<Loaded<T> | null>(null);

  // The latest loader, synced in an effect so it is never touched during render.
  const loadRef = useRef(load);
  useEffect(() => {
    loadRef.current = load;
  });

  const fullKey = key === null ? null : `${identity}\n${key}\n${tick}`;

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

  const current = loaded !== null && loaded.key === fullKey ? loaded : null;
  return {
    data: current?.data ?? null,
    error: current?.error ?? null,
    loading: fullKey !== null && current === null,
    reload,
    setData,
  };
}
