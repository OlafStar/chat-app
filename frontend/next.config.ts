import type { NextConfig } from "next";

const CLIENT_API_ORIGIN =
  process.env.CLIENT_API_ORIGIN ?? "http://localhost:81";

const nextConfig: NextConfig = {
  output: "standalone",
  async rewrites() {
    return [
      {
        source: "/api/client/v1/:path*",
        destination: `${CLIENT_API_ORIGIN}/api/client/v1/:path*`,
      },
    ];
  },
};

export default nextConfig;
