const backend = process.env.BACKEND_URL || "http://localhost:8080";

const nextConfig = {
  output: "standalone",
  experimental: {
    middlewareClientMaxBodySize: "65mb",
    proxyTimeout: 120_000,
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
