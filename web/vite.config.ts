import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import path from "path";

export default defineConfig({
  base: "/pgpageshell/",
  plugins: [react()],
  resolve: {
    alias: {
      "@shared": path.resolve(__dirname, "../frontend/src"),
    },
    dedupe: ["react", "react-dom"],
  },
  optimizeDeps: {
    include: ["react", "react-dom"],
  },
});
