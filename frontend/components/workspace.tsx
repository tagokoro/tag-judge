"use client";

import { createContext, useContext } from "react";
import type { JudgeClient } from "@/lib/api";
import type { Me, Role } from "@/lib/types";

/** Shared state for the whole SPA. Provided by app/page.tsx. */
export interface WorkspaceValue {
  client: JudgeClient;

  // Authentication / session
  session: Me | null; // active cookie session, null = none
  cookieChecked: boolean;
  devEnabled: boolean;
  devUserId: string;
  devRole: Role;
  setDevEnabled: (value: boolean) => void;
  setDevUserId: (value: string) => void;
  setDevRole: (value: Role) => void;
  switchDevUser: () => void;
  refreshSession: () => Promise<void>;
  logout: () => Promise<void>;

  // Selection shared across panels
  namespaceSlug: string;
  setNamespaceSlug: (value: string) => void;
  librarySlug: string;
  setLibrarySlug: (value: string) => void;
  versionNumber: number | null;
  setVersionNumber: (value: number | null) => void;

  // Incremented on an explicit "switch user" so panels can re-fetch (MVP §11).
  refreshTick: number;
}

export const WorkspaceContext = createContext<WorkspaceValue | null>(null);

export function useWorkspace(): WorkspaceValue {
  const value = useContext(WorkspaceContext);
  if (!value) {
    throw new Error("useWorkspace must be used inside WorkspaceContext.Provider");
  }
  return value;
}
