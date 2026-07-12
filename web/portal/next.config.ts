import type { NextConfig } from "next";

const nextConfig: NextConfig = {
  // "standalone" emits a self-contained server bundle for a small Docker image
  // (used by docker/portal/Dockerfile in Part C).
  output: "standalone",
  reactStrictMode: true,
};

export default nextConfig;
