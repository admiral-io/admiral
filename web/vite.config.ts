import { createLogger as createLoggerRaw, defineConfig, loadEnv } from 'vite';
import react from '@vitejs/plugin-react';

export default defineConfig(({ mode }) => {
  process.env = { ...process.env, ...loadEnv(mode, process.cwd()) };

  const logger = {
    base: createLoggerRaw('info', { prefix: '[vite-proxy]', allowClearScreen: false }),
    info: (msg: string) => {
      logger.base.info(msg, { timestamp: true, clear: false });
    },
    warn: (msg: string) => {
      logger.base.warn(msg, { timestamp: true, clear: false });
    },
    error: (msg: string) => {
      logger.base.error(msg, { timestamp: true, clear: false });
    },
  };

  return {
    build: {
      manifest: true,
      sourcemap: false,
      outDir: 'build',
    },
    server: {
      host: 'localhost',
      port: 8888,
      proxy: {
        '^/((auth|api|debug)/.*)|(meta|metrics|healthcheck)$': {
          target: process.env.VITE_API_ENDPOINT || 'http://localhost:8080',
          changeOrigin: true,
          xfwd: true,
          secure: false,
          configure: (proxy) => {
            // eslint-disable-next-line @typescript-eslint/no-unused-vars
            proxy.on('error', (err, _req, _res) => {
              logger.error(`error when starting dev server:\n${err.stack}`);
            });
            // eslint-disable-next-line @typescript-eslint/no-unused-vars
            proxy.on('proxyReq', (_proxyReq, req, _res) => {
              logger.info(`request - ${req.method} ${req.url}`);
            });
            // eslint-disable-next-line @typescript-eslint/no-unused-vars
            proxy.on('proxyRes', (proxyRes, req, _res) => {
              logger.info(`response - ${req.method} ${req.url} ${proxyRes.statusCode}`);
            });
          },
        },
      },
    },
    plugins: [react()],
  };
});