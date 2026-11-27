import type { NextConfig } from "next";
import { apiServerOrigin } from "./lib/apiOrigin";

const nextConfig: NextConfig = {
  // The EC2 instance runs .next/standalone/server.js with a bare Node.js, so
  // the build carries only the files the server needs. API_SERVER_ORIGIN must
  // be set at build time too: the rewrites below are baked into the output.
  output: "standalone",
  async redirects() {
    return [
      {
        // The MCP URL is what users paste into Claude, so some open it in a
        // browser too. A page load asks for HTML; MCP clients never do, so
        // only people are sent to the setup guide instead of a bare 401.
        source: "/mcp",
        has: [{ type: "header", key: "accept", value: "(.*)text/html(.*)" }],
        destination: "/connect",
        permanent: false,
      },
    ];
  },
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
      // The MCP endpoint and its OAuth authorization server also live in the
      // Go API. Serving them on this origin keeps the consent page on the
      // origin that holds the session cookie.
      { source: "/mcp", destination: `${apiServerOrigin()}/mcp` },
      { source: "/oauth/:path*", destination: `${apiServerOrigin()}/oauth/:path*` },
      {
        source: "/.well-known/oauth-protected-resource/:path*",
        destination: `${apiServerOrigin()}/.well-known/oauth-protected-resource/:path*`,
      },
      {
        source: "/.well-known/oauth-authorization-server",
        destination: `${apiServerOrigin()}/.well-known/oauth-authorization-server`,
      },
    ];
  },
};

export default nextConfig;
