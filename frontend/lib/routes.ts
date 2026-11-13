// Page URL builders. User pages live under /users/ so a handle can never
// collide with an app route such as /me or with the /api rewrite. Categories
// and libraries are found by their random id, under their owner's handle
// (like Qiita's /{user}/items/{id}); a library's URL does not name its
// category, so moving it keeps the URL. Each segment is percent-encoded.

export const HOME_PATH = "/";
export const ME_PATH = "/me";
export const PROBLEMS_PATH = "/problems";
export const SUBMISSIONS_PATH = "/submissions";
export const MY_PROBLEMS_PATH = "/me/problems";
export const MY_SUBMISSIONS_PATH = "/me/submissions";
/** Where a signed-in user without a handle registers one. */
export const SIGNUP_PATH = "/signup";
export const SETTINGS_PROFILE_PATH = "/settings/profile";

/** Creates a library; with a category id, the form starts in that category. */
export function newLibraryPath(categoryId?: string): string {
  return categoryId ? `/libraries/new?category=${encodeURIComponent(categoryId)}` : "/libraries/new";
}

export function userPath(handle: string): string {
  return `/users/${encodeURIComponent(handle)}`;
}

export function categoryPath(handle: string, categoryId: string): string {
  return `${userPath(handle)}/categories/${encodeURIComponent(categoryId)}`;
}

/** Renames or describes a category. Categories are otherwise kept by their libraries. */
export function categoryEditPath(handle: string, categoryId: string): string {
  return `${categoryPath(handle, categoryId)}/edit`;
}

export function libraryPath(handle: string, libraryId: string): string {
  return `${userPath(handle)}/libraries/${encodeURIComponent(libraryId)}`;
}

// Edit pages are separate from the read-only pages, even for the owner.
export function libraryEditPath(handle: string, libraryId: string): string {
  return `${libraryPath(handle, libraryId)}/edit`;
}

/** Replaces the library's code. The server keeps each upload, but pages show only the latest. */
export function libraryCodeEditPath(handle: string, libraryId: string): string {
  return `${libraryPath(handle, libraryId)}/code/edit`;
}

/** Sets or clears the submission URL of the library's latest code. */
export function submissionUrlEditPath(handle: string, libraryId: string): string {
  return `${libraryPath(handle, libraryId)}/submission-url/edit`;
}
