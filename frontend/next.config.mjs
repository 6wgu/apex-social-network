// The browser only ever talks to the Next.js server (port 3000). These rewrites
// proxy API, uploads and WebSocket traffic to the Go backend, so the session
// cookie is same-origin and needs no CORS. BACKEND_URL is read at build time.
const backend = process.env.BACKEND_URL || "http://localhost:8080";

/** @type {import('next').NextConfig} */
const nextConfig = {
  // Builds .next/standalone: a minimal server.js plus only the node_modules it
  // needs, so the Docker runtime image does not carry the full dependency tree.
  output: "standalone",
  experimental: {
    // The proxy cuts request bodies off at 10 MB by default, which turns a
    // too-big upload into a broken request. Let it through (the backend's own
    // cap is 32 MB) so the backend can answer with its "5 MB or smaller" error.
    middlewareClientMaxBodySize: "33mb",
  },
  async rewrites() {
    return [
      { source: "/api/:path*", destination: `${backend}/api/:path*` },
      { source: "/uploads/:path*", destination: `${backend}/uploads/:path*` },
      { source: "/ws", destination: `${backend}/ws` },
    ];
  },
};

export default nextConfig;
