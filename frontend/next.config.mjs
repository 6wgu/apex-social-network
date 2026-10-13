// The browser only ever talks to the Next.js server (port 3000). These rewrites
// proxy API, uploads and WebSocket traffic to the Go backend, so the session
// cookie is same-origin and needs no CORS. BACKEND_URL is read at build time.
const backend = process.env.BACKEND_URL || "http://localhost:8080";

/** @type {import('next').NextConfig} */
const nextConfig = {
  async rewrites() {
    return [
      { source: "/api/:path*", destination: `${backend}/api/:path*` },
      { source: "/uploads/:path*", destination: `${backend}/uploads/:path*` },
      { source: "/ws", destination: `${backend}/ws` },
    ];
  },
};

export default nextConfig;
