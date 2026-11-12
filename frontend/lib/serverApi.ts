// Server-side data fetching for Server Components. Requests go straight to the
// Go API (not through the /api rewrite) and carry the visitor's cookies, so a
// signed-in owner sees their private libraries on the first render. Dev header
// auth lives only in the browser and is never sent from here.

import { cache } from "react";
import { headers } from "next/headers";
import { createJudgeClient, isApiRequestError, type JudgeClient } from "./api";
import { apiServerOrigin } from "./apiOrigin";
import type { Library, LibraryVersion, Namespace } from "./types";

/** A client bound to this request's cookies. Never cached across requests. */
export const serverClient = cache(async (): Promise<JudgeClient> => {
  const cookie = (await headers()).get("cookie");
  const origin = apiServerOrigin();
  const fetchImpl: typeof fetch = (input, init) =>
    fetch(`${origin}${String(input)}`, {
      ...init,
      cache: "no-store",
      headers: {
        ...(init?.headers as Record<string, string> | undefined),
        ...(cookie ? { cookie } : {}),
      },
    });
  return createJudgeClient(() => null, fetchImpl);
});

/** Resolves to null on a 404 (missing, or private to someone else); other errors throw. */
export async function orNull<T>(request: Promise<T>): Promise<T | null> {
  try {
    return await request;
  } catch (error) {
    if (isApiRequestError(error) && error.status === 404) {
      return null;
    }
    throw error;
  }
}

// Cached per request so generateMetadata and the page share one fetch.

export const getNamespace = cache(async (namespaceSlug: string): Promise<Namespace | null> =>
  orNull((await serverClient()).getNamespace(namespaceSlug)),
);

export const getLibrary = cache(
  async (namespaceSlug: string, librarySlug: string): Promise<Library | null> =>
    orNull((await serverClient()).getLibrary(namespaceSlug, librarySlug)),
);

/** The library's latest code, or null when it has none. */
export const getLatestVersion = cache(
  async (namespaceSlug: string, librarySlug: string): Promise<LibraryVersion | null> => {
    const api = await serverClient();
    const [latest] = await api.listVersions(namespaceSlug, librarySlug, 1);
    return latest ? api.getVersion(namespaceSlug, librarySlug, latest.version_number) : null;
  },
);
