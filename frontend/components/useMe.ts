"use client";

import { useAuth } from "./AuthProvider";
import { useResource } from "./useResource";

/**
 * The signed-in user's /me, asked as the current actor (so a dev-header user
 * gets their own handle). Pages that act on "no handle yet" wait for it.
 */
export function useMe() {
  const { actor } = useAuth();
  return useResource(actor ? "me" : null, (api) => api.getMe());
}
