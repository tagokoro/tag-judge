// Page URL builders. Namespace pages live under /n/ so a namespace slug can
// never collide with an app route such as /me or with the /api rewrite. Each
// slug is percent-encoded as one path segment.

export const HOME_PATH = "/";
export const ME_PATH = "/me";
export const PROBLEMS_PATH = "/problems";
export const SUBMISSIONS_PATH = "/submissions";
export const MY_PROBLEMS_PATH = "/me/problems";
export const MY_SUBMISSIONS_PATH = "/me/submissions";
/** Where a signed-in user without a handle registers one. */
export const SIGNUP_PATH = "/signup";
export const SETTINGS_PROFILE_PATH = "/settings/profile";
/** Creating a library always targets the signed-in user's own namespace. */
export const NEW_LIBRARY_PATH = "/libraries/new";

/** A user's own page, under /users/ so a handle never collides with an app route. */
export function userPath(handle: string): string {
  return `/users/${encodeURIComponent(handle)}`;
}

export function namespacePath(namespaceSlug: string): string {
  return `/n/${encodeURIComponent(namespaceSlug)}`;
}

export function libraryPath(namespaceSlug: string, librarySlug: string): string {
  return `${namespacePath(namespaceSlug)}/${encodeURIComponent(librarySlug)}`;
}

// Edit pages are separate from the read-only pages, even for the owner.
export function libraryEditPath(namespaceSlug: string, librarySlug: string): string {
  return `${libraryPath(namespaceSlug, librarySlug)}/edit`;
}

/** Replaces the library's code. The server keeps each upload, but pages show only the latest. */
export function libraryCodeEditPath(namespaceSlug: string, librarySlug: string): string {
  return `${libraryPath(namespaceSlug, librarySlug)}/code/edit`;
}

/** Sets or clears the submission URL of the library's latest code. */
export function submissionUrlEditPath(namespaceSlug: string, librarySlug: string): string {
  return `${libraryPath(namespaceSlug, librarySlug)}/submission-url/edit`;
}
