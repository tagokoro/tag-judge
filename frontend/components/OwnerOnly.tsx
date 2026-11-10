"use client";

import Link from "next/link";
import type { ReactNode } from "react";
import { isOwner } from "@/lib/session";
import { useAuth } from "./AuthProvider";
import { useResource } from "./useResource";
import { LoadState } from "./ui";

/** The namespace and whether the current actor owns it. */
export function useNamespaceOwner(namespaceSlug: string) {
  const { actor } = useAuth();
  const namespace = useResource(`ns:${namespaceSlug}`, (api) => api.getNamespace(namespaceSlug));
  const owner = namespace.data !== null && isOwner(actor, namespace.data.owner_user_id);
  return { namespace, owner };
}

/**
 * Wraps an edit page: only the namespace owner sees the form. The API still
 * enforces ownership; this only avoids showing a form that would be refused.
 */
export function OwnerOnly({
  namespaceSlug,
  backHref,
  children,
}: {
  namespaceSlug: string;
  backHref: string;
  children: ReactNode;
}) {
  const { namespace, owner } = useNamespaceOwner(namespaceSlug);
  if (!namespace.data) {
    return (
      <LoadState loading={namespace.loading} error={namespace.error} notFound="この Namespace は見つかりません。" />
    );
  }
  if (!owner) {
    return (
      <p className="empty">
        このページは所有者だけが使えます。<Link href={backHref}>表示ページに戻る</Link>
      </p>
    );
  }
  return <>{children}</>;
}
