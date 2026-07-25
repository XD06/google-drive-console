/// <reference types="vitest/config" />
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  server: {
    port: 5174,
    proxy: {
      "/api": "http://localhost:3000",
      "/oauth2": "http://localhost:3000",
    },
  },
  build: {
    outDir: "dist",
    emptyOutDir: true,
    // Modern browsers only: smaller/faster output, no legacy transforms.
    target: "es2020",
    // Skip gzip size reporting to speed up the build itself.
    reportCompressedSize: false,
    chunkSizeWarningLimit: 700,
    rollupOptions: {
      output: {
        // Split the stable React runtime into its own long-cacheable chunk so
        // app code changes don't bust the whole bundle's browser cache.
        // Function form catches react-dom/client and jsx-runtime subpaths.
        manualChunks(id) {
          if (/node_modules[\\/](react|react-dom|scheduler)[\\/]/.test(id)) {
            return "react";
          }
        },
      },
    },
  },
  test: {
    environment: "jsdom",
    globals: true,
    setupFiles: "./src/test/setup.ts",
  },
});
