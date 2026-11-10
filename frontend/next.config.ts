import type { NextConfig } from "next";

// Local development fallback for the Go API process. The Go API listens on
// localhost:8080 and the Next.js dev server on localhost:3000 (see README).
const DEV_API_SERVER_ORIGIN = "http://localhost:8080";

const nextConfig: NextConfig = {
  async rewrites() {
    // API_SERVER_ORIGIN is intentionally read WITHOUT a NEXT_PUBLIC_ prefix.
    // It stays server-side only: the rewrite destination is resolved by the
    // Next.js server, so the browser never learns the Go API origin and only
    // ever calls same-origin /api/* paths. Production deployments must set
    // API_SERVER_ORIGIN; local development falls back to localhost:8080.
    const apiServerOrigin = (
      process.env.API_SERVER_ORIGIN || DEV_API_SERVER_ORIGIN
    ).replace(/\/+$/, "");

    return [
      {
        // A single catch-all also covers the Google OAuth callback
        // (/api/v1/auth/google/callback), so the browser-facing callback stays
        // on the Next.js origin and the existing Go cookie session keeps working.
        source: "/api/:path*",
        destination: `${apiServerOrigin}/api/:path*`,
      },
    ];
  },
};

export default nextConfig;
