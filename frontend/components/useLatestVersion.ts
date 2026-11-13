"use client";

import type { JudgeClient } from "@/lib/api";
import type { LibraryVersion } from "@/lib/types";
import { useResource } from "./useResource";

export interface LatestVersion {
  /** The latest code, or null when the library has none yet (or while loading). */
  data: LibraryVersion | null;
  /** True once it is known that the library has no code. */
  empty: boolean;
  loading: boolean;
  error: unknown;
}

/** The number of the library's latest version, or null when it has none. */
export async function fetchLatestVersionNumber(api: JudgeClient, libraryId: string): Promise<number | null> {
  const [latest] = await api.listVersions(libraryId, 1);
  return latest?.version_number ?? null;
}

/**
 * Loads a library's latest code. Pages never pick an older upload: a
 * one-row version list (newest first) only tells which one is the latest.
 */
export function useLatestVersion(
  libraryId: string,
  /** The latest code the server already fetched (null: none). */
  initial?: { data: LibraryVersion | null },
): LatestVersion {
  const latestNumber = useResource(
    `lib:${libraryId}:latest`,
    (api) => fetchLatestVersionNumber(api, libraryId),
    initial && { data: initial.data?.version_number ?? null },
  );
  const latest = latestNumber.data;
  const version = useResource(
    latest === null ? null : `ver:${libraryId}/${latest}`,
    (api) => api.getVersion(libraryId, latest ?? 0),
    initial?.data ? { data: initial.data } : undefined,
  );
  return {
    data: version.data,
    // useResource maps a loaded null to data null too, so "no code" is "loaded without error".
    empty: latest === null && !latestNumber.loading && latestNumber.error === null,
    loading: latestNumber.loading || version.loading,
    error: latestNumber.error ?? version.error,
  };
}
