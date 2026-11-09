"use client";

import { useCallback, useState } from "react";
import { formatError } from "./ui";

/** Small helper: tracks busy/error state for one-off async button actions. */
export function useAsyncAction() {
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const run = useCallback(async (action: () => Promise<void>): Promise<void> => {
    setBusy(true);
    setError(null);
    try {
      await action();
    } catch (caught) {
      setError(formatError(caught));
    } finally {
      setBusy(false);
    }
  }, []);

  return { busy, error, setError, run };
}
