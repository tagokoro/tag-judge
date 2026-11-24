import type { NextConfig } from "next";
import { apiServerOrigin } from "./lib/apiOrigin";

const nextConfig: NextConfig = {
  // The EC2 instance runs .next/standalone/server.js with a bare Node.js, so
  // the build carries only the files the server needs. API_SERVER_ORIGIN must
  // be set at build time too: the rewrites below are baked into the output.
  output: "standalone",
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
