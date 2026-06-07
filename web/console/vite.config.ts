import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  base: "/console/",
  server: {
    port: 5173,
    proxy: {
      "/api": {
        target: process.env.INFINITY_QUERIER_URL ?? "http://127.0.0.1:8808",
        changeOrigin: true
      }
    }
  }
});
