"use client";

import { createContext, useContext } from "react";
import type { Dispatch } from "react";
import type { JudgeClient } from "@/lib/api";
import type {
  ScopedData,
  WorkspaceAction,
  WorkspaceState,
} from "@/lib/workspaceState";
import type { Role } from "@/lib/types";

/** Shared state for the whole SPA. Provided by app/page.tsx. */
export interface WorkspaceValue {
  client: JudgeClient;

  // Whole state lives in one reducer so an actor change clears private data in
  // a single place. Panels read state.data.* / state.namespaceSlug / etc.
  state: WorkspaceState;
  dispatch: Dispatch<WorkspaceAction>;
  effectiveVersion: number | null;

  // Authentication / session
  setDevEnabled: (value: boolean) => void;
  setDevUserId: (value: string) => void;
  setDevRole: (value: Role) => void;
  switchDevUser: () => void;
  refreshSession: () => Promise<void>;
  logout: () => Promise<void>;

  // Selection shared across panels. The optional `patch` makes a selection
  // change + the data loaded for the NEW target ATOMIC (see workspaceState):
  // the reducer clears stale version/library/link data, bumps the selection
  // generation, then applies the patch in one dispatch. Pure navigation (typing
  // a slug) omits the patch and always applies.
  setNamespaceSlug: (value: string, patch?: Partial<ScopedData>) => void;
  setLibrarySlug: (value: string, patch?: Partial<ScopedData>) => void;
  setVersion: (value: number | null, patch?: Partial<ScopedData>) => void;

  // Data helpers. patchData is guarded by authGeneration AND
  // selectionGeneration: a result from a fetch started under a previous actor
  // OR a previous target is dropped by the reducer. This also applies to the
  // selected submission link (currentLink) and the link list (links).
  patchData: (patch: Partial<ScopedData>) => void;
  clearLibraryDetail: () => void;
}

export const WorkspaceContext = createContext<WorkspaceValue | null>(null);

export function useWorkspace(): WorkspaceValue {
  const value = useContext(WorkspaceContext);
  if (!value) {
    throw new Error("useWorkspace must be used inside WorkspaceContext.Provider");
  }
  return value;
}
