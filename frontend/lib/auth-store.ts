"use client";

import type { DevAuth, Role } from "@/lib/types";

const storageKey = "tag-judge-dev-auth";
const eventName = "tag-judge-dev-auth-change";

export const defaultDevAuth: DevAuth = {
  userId: "00000000-0000-4000-8000-000000000001",
  role: "author"
};

export function readDevAuth(): DevAuth | null {
  if (typeof window === "undefined") {
    return null;
  }
  const raw = window.localStorage.getItem(storageKey);
  if (!raw) {
    return null;
  }
  try {
    const value = JSON.parse(raw) as Partial<DevAuth>;
    if (!value.userId || !isRole(value.role)) {
      return null;
    }
    return { userId: value.userId, role: value.role };
  } catch {
    return null;
  }
}

export function writeDevAuth(auth: DevAuth | null) {
  if (typeof window === "undefined") {
    return;
  }
  if (auth) {
    window.localStorage.setItem(storageKey, JSON.stringify(auth));
  } else {
    window.localStorage.removeItem(storageKey);
  }
  window.dispatchEvent(new Event(eventName));
}

export function subscribeDevAuth(callback: () => void) {
  window.addEventListener(eventName, callback);
  window.addEventListener("storage", callback);
  return () => {
    window.removeEventListener(eventName, callback);
    window.removeEventListener("storage", callback);
  };
}

function isRole(value: unknown): value is Role {
  return value === "user" || value === "author" || value === "admin";
}
