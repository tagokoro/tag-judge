"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { formatError } from "./ui";

/**
 * Tracks busy/error for one-off async button actions and owns an AbortController.
 * On unmount (which happens on every actor/auth change because the panels are
 * keyed by authGeneration) the in-flight request is aborted, so outstanding
 * fetches cannot land after private data has been cleared.
 */
export function useAsyncAction() {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const controllerRef = useRef<AbortController | null>(null);

  useEffect(
    () => () => {
      controllerRef.current?.abort();
    },
    [],
  );

  const run = useCallback(
    async (action: (signal: AbortSignal) => Promise<void>): Promise<void> => {
      const controller = new AbortController();
      controllerRef.current = controller;
      setBusy(true);
      setError(null);
      try {
        await action(controller.signal);
      } catch (caught) {
        // A cancelled request is not a user-visible failure.
        if (!controller.signal.aborted) {
          setError(formatError(caught));
        }
      } finally {
        // Only the most recent action may clear the busy flag.
        if (controllerRef.current === controller) {
          setBusy(false);
        }
      }
    },
    [],
  );

  return { busy, error, setError, run };
}
