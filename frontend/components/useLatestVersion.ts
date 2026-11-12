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
export async function fetchLatestVersionNumber(
  api: JudgeClient,
  namespaceSlug: string,
  librarySlug: string,
): Promise<number | null> {
  const [latest] = await api.listVersions(namespaceSlug, librarySlug, 1);
  return latest?.version_number ?? null;
}

/**
 * Loads a library's latest code. Pages never pick an older upload: a
 * one-row version list (newest first) only tells which one is the latest.
 */
export function useLatestVersion(namespaceSlug: string, librarySlug: string): LatestVersion {
  const latestNumber = useResource(`lib:${namespaceSlug}/${librarySlug}:latest`, (api) =>
    fetchLatestVersionNumber(api, namespaceSlug, librarySlug),
  );
  const latest = latestNumber.data;
  const version = useResource(
    latest === null ? null : `ver:${namespaceSlug}/${librarySlug}/${latest}`,
    (api) => api.getVersion(namespaceSlug, librarySlug, latest ?? 0),
  );
  return {
    data: version.data,
    // useResource maps a loaded null to data null too, so "no code" is "loaded without error".
    empty: latest === null && !latestNumber.loading && latestNumber.error === null,
    loading: latestNumber.loading || version.loading,
    error: latestNumber.error ?? version.error,
  };
}
