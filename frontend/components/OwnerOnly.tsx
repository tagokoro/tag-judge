"use client";

import { notFound } from "next/navigation";
import type { ReactNode } from "react";
import { isApiRequestError, notFoundError } from "@/lib/api";
import { isOwner } from "@/lib/session";
import type { Library, Category } from "@/lib/types";
import { useAuth } from "./AuthProvider";
import { useResource, type Resource } from "./useResource";
import { LoadState } from "./ui";

/** Fails like a missing resource when the URL's handle is not the owner. */
async function ownedBy<T extends { owner: string }>(handle: string, request: Promise<T>): Promise<T> {
  const resource = await request;
  if (resource.owner !== handle) {
    throw notFoundError();
  }
  return resource;
}

/** The category a /users/{handle}/categories/{id} URL names. */
export function useUserCategory(handle: string, categoryId: string, initial?: { data: Category }) {
  return useResource(
    `category:${categoryId}`,
    (api) => ownedBy(handle, api.getCategory(categoryId)),
    initial,
  );
}

/** The library a /users/{handle}/libraries/{id} URL names. */
export function useUserLibrary(handle: string, libraryId: string, initial?: { data: Library }) {
  return useResource(
    `lib:${libraryId}`,
    (api) => ownedBy(handle, api.getLibrary(libraryId)),
    initial,
  );
}

/** Whether the current actor owns a loaded category or library. */
export function useIsOwner(resource: { owner: string } | null): boolean {
  const { actor } = useAuth();
  return resource !== null && isOwner(actor, resource.owner);
}

/**
 * Wraps an edit page: only the owner sees it. Anyone else gets the same 404 as
 * a missing page, matching the API (a private library is a 404 to non-owners),
 * so the page never confirms what exists. The API still enforces ownership on
 * every write.
 */
export function OwnerOnly<T extends { owner: string }>({
  resource,
  children,
}: {
  resource: Resource<T>;
  children: (data: T) => ReactNode;
}) {
  const owner = useIsOwner(resource.data);
  if (isApiRequestError(resource.error) && resource.error.status === 404) {
    notFound();
  }
  if (!resource.data) {
    return <LoadState loading={resource.loading} error={resource.error} notFound="" />;
  }
  if (!owner) {
    notFound();
  }
  return <>{children(resource.data)}</>;
}
