import type { NextConfig } from "next";

const isDev = process.env.NODE_ENV === 'development';

const apiProxyTarget =
  process.env.API_PROXY_TARGET?.trim() ||
  process.env.NEXT_PUBLIC_API_BASE?.trim() ||
  "http://localhost:8080";

const extraAllowedDevOrigins =
  process.env.NEXT_ALLOWED_DEV_ORIGINS?.split(',')
    .map((origin) => origin.trim())
    .filter(Boolean) ?? [];

const nextConfig: NextConfig = {
  output: isDev ? undefined : 'export',
  images: { unoptimized: true },
  allowedDevOrigins: ['127.0.0.1', 'localhost', ...extraAllowedDevOrigins],
  ...(isDev
    ? {
        async rewrites() {
          return {
            beforeFiles: [],
            afterFiles: [],
            fallback: [
              {
                source: '/api/:path*',
                destination: `${apiProxyTarget}/api/:path*`,
              },
            ],
          };
        },
      }
    : {}),
};

export default nextConfig;
