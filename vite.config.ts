import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import relay from "vite-plugin-relay";

export default defineConfig({
  plugins: [
    react({
      babel: {
        plugins: ["relay"],
      },
    }),
    relay,
  ],
  build: {
    outDir: "pkg/web/dist",
    emptyOutDir: true,
    assetsDir: "assets",
  },
  server: {
    proxy: {
      "/graphql": "http://127.0.0.1:8080",
    },
  },
  base: "/",
});
