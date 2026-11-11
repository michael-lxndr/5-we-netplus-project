import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";

export default defineConfig({
  plugins: [react()],
  server: {
    host: "0.0.0.0",
    watch: {
      // Docker bind mounts on Windows may not forward file change events.
      usePolling: true,
    },
    proxy: {
      "/api": "http://api:8080",
    },
  },
});
