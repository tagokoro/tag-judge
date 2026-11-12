"use client";

import { notFound } from "next/navigation";
import type { ReactNode } from "react";
import { isApiRequestError } from "@/lib/api";
import { isOwner } from "@/lib/session";
import type { Namespace } from "@/lib/types";
import { useAuth } from "./AuthProvider";
import { useResource } from "./useResource";
import { LoadState } from "./ui";

/** The namespace and whether the current actor owns it. */
export function useNamespaceOwner(namespaceSlug: string, initial?: { data: Namespace }) {
  const { actor } = useAuth();
  const namespace = useResource(`ns:${namespaceSlug}`, (api) => api.getNamespace(namespaceSlug), initial);
  const owner = namespace.data !== null && isOwner(actor, namespace.data.owner_user_id);
  return { namespace, owner };
}

/**
 * Wraps an edit page: only the namespace owner sees it. Anyone else gets the
 * same 404 as a missing page, matching the API (a private library is a 404 to
 * non-owners), so the page never confirms what exists. The API still enforces
 * ownership on every write.
 */
export function OwnerOnly({ namespaceSlug, children }: { namespaceSlug: string; children: ReactNode }) {
  const { namespace, owner } = useNamespaceOwner(namespaceSlug);
  if (isApiRequestError(namespace.error) && namespace.error.status === 404) {
    notFound();
  }
  if (!namespace.data) {
    return <LoadState loading={namespace.loading} error={namespace.error} notFound="" />;
  }
  if (!owner) {
    notFound();
  }
  return <>{children}</>;
}
