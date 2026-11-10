// Page URL builders. Namespace pages live under /n/ so a namespace slug can
// never collide with an app route such as /me or with the /api rewrite. Each
// slug is percent-encoded as one path segment.

export const HOME_PATH = "/";
export const ME_PATH = "/me";
export const PROBLEMS_PATH = "/problems";
export const SUBMISSIONS_PATH = "/submissions";
export const MY_PROBLEMS_PATH = "/me/problems";
export const MY_SUBMISSIONS_PATH = "/me/submissions";

/** Mirrors the backend's int32 cap on version_number. */
const MAX_VERSION_NUMBER = 2147483647;

export function namespacePath(namespaceSlug: string): string {
  return `/n/${encodeURIComponent(namespaceSlug)}`;
}

export function libraryPath(namespaceSlug: string, librarySlug: string): string {
  return `${namespacePath(namespaceSlug)}/${encodeURIComponent(librarySlug)}`;
}

export function versionPath(
  namespaceSlug: string,
  librarySlug: string,
  versionNumber: number,
): string {
  return `${libraryPath(namespaceSlug, librarySlug)}/v/${versionNumber}`;
}

/** Parses the [version] route param; null for anything but 1..int32 max. */
export function parseVersionParam(raw: string): number | null {
  if (!/^[1-9][0-9]*$/.test(raw)) {
    return null;
  }
  const value = Number(raw);
  return value <= MAX_VERSION_NUMBER ? value : null;
}
