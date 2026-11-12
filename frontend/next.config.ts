import type { NextConfig } from "next";
import { apiServerOrigin } from "./lib/apiOrigin";

const nextConfig: NextConfig = {
  async rewrites() {
    return [
      {
        // The browser only ever calls same-origin /api/* paths. A single
        // catch-all also covers the Google OAuth callback
        // (/api/v1/auth/google/callback), so the browser-facing callback stays
        // on the Next.js origin and the existing Go cookie session keeps working.
        source: "/api/:path*",
        destination: `${apiServerOrigin()}/api/:path*`,
      },
    ];
  },
};

export default nextConfig;
