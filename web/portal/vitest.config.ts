import { defineConfig } from "vitest/config";
import react from "@vitejs/plugin-react";
import { resolve } from "node:path";

// Vitest runs component/unit tests in a simulated browser (jsdom). Playwright
// (separate config) drives the real built app for e2e.
export default defineConfig({
  plugins: [react()],
  test: {
    environment: "jsdom",
    globals: true,
    setupFiles: ["./vitest.setup.ts"],
    // e2e specs are Playwright's, not Vitest's.
    exclude: ["node_modules", "e2e", ".next"],
  },
  resolve: {
    alias: {
      "@": resolve(__dirname, "."),
    },
  },
});
