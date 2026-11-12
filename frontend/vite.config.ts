import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

export default defineConfig(() => {
  // La API en Go corre en el host (ver mise.toml); /api se proxifica hacia ella.
  const proxy = {
    "/api": { target: process.env.API_PROXY_TARGET ?? "http://localhost:8080" },
  };
  return {
    plugins: [react(), tailwindcss()],
    server: { proxy },
    preview: { proxy },
    build: {
      rollupOptions: {
        output: {
          manualChunks: { react: ["react", "react-dom", "react-router-dom"] },
        },
      },
    },
  };
});
