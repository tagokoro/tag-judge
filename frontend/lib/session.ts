// Pure rules for who the current actor is. A real cookie session always wins;
// dev header auth is used only when explicitly enabled and no session exists.
// No React and no runtime imports from other local modules, so node:test can
// load this file directly with TypeScript stripping.

import type { DevHeaders, Me, Role } from "./types";

export interface DevUser {
  userId: string;
  role: Role;
}

export interface SessionState {
  session: Me | null;
  cookieChecked: boolean;
  devEnabled: boolean;
  devUser: DevUser;
}

/** Seed users from backend/db/seeds/development.sql, for dev header auth. */
export const SEED_USERS: readonly DevUser[] = [
  { userId: "00000000-0000-4000-8000-000000000001", role: "author" },
  { userId: "00000000-0000-4000-8000-000000000002", role: "admin" },
];

export interface Actor {
  userId: string;
  role: Role;
  source: "cookie" | "dev";
}

export function actorOf(state: SessionState): Actor | null {
  if (state.session) {
    return { userId: state.session.user_id, role: state.session.role, source: "cookie" };
  }
  if (state.devEnabled) {
    return { userId: state.devUser.userId, role: state.devUser.role, source: "dev" };
  }
  return null;
}

/**
 * A stable key for the effective actor. Pages remount when it changes, so data
 * loaded for a previous actor (including private sources) is never shown to
 * the next one and in-flight requests are aborted.
 */
export function identityOf(state: SessionState): string {
  const actor = actorOf(state);
  return actor ? `${actor.source}:${actor.userId}:${actor.role}` : "anon";
}

export function devHeadersOf(state: SessionState): DevHeaders | null {
  const actor = actorOf(state);
  if (!actor || actor.source !== "dev") {
    return null;
  }
  return { "X-User-ID": actor.userId, "X-User-Role": actor.role };
}

/** Owner check for showing owner-only controls; the API still enforces it. */
export function isOwner(actor: Actor | null, ownerUserId: string): boolean {
  return actor !== null && actor.userId.toLowerCase() === ownerUserId.toLowerCase();
}

/** Dev header auth exists only outside production builds. */
export const DEV_AUTH_AVAILABLE = process.env.NODE_ENV !== "production";
