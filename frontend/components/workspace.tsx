"use client";

import { createContext, useContext } from "react";
import type { Dispatch } from "react";
import type { JudgeClient } from "@/lib/api";
import type {
  ScopedData,
  VerificationView,
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

  // Selection shared across panels
  setNamespaceSlug: (value: string) => void;
  setLibrarySlug: (value: string) => void;
  setVersion: (value: number | null) => void;

  // Data helpers. patchData is guarded by authGeneration: a result from a fetch
  // started under a previous actor is dropped by the reducer.
  patchData: (patch: Partial<ScopedData>) => void;
  selectVerification: (value: VerificationView) => void;
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
