// Where the Go API lives, as seen from the Next.js server. Shared by the
// /api rewrite (next.config.ts) and server-side data fetching (lib/serverApi.ts).
//
// API_SERVER_ORIGIN is intentionally read WITHOUT a NEXT_PUBLIC_ prefix: it
// stays server-side only, so the browser never learns the Go API origin.
// Production deployments must set it; local development falls back to the
// Go API's default port.

const DEV_API_SERVER_ORIGIN = "http://localhost:8080";

export function apiServerOrigin(): string {
  return (process.env.API_SERVER_ORIGIN || DEV_API_SERVER_ORIGIN).replace(/\/+$/, "");
}
