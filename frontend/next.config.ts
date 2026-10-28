import type { NextConfig } from "next";
import path from "node:path";

const API_ORIGIN = process.env.API_PROXY_ORIGIN || "http://127.0.0.1:8080";

const nextConfig: NextConfig = {
  output: "standalone",
  outputFileTracingRoot: path.join(process.cwd()),
  // 允许通过环境变量指定构建目录，便于在受限环境里做并行验证而不影响 .next
  distDir: process.env.NEXT_DIST_DIR || ".next",
  // 前端默认同源调用 /api，由 Next 反代到后端，避免跨域与环境代理干扰
  async rewrites() {
    return [{ source: "/api/:path*", destination: `${API_ORIGIN}/api/:path*` }];
  },
};

export default nextConfig;
