import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
import path from "node:path";
import { fileURLToPath } from "node:url";

const __dirname = path.dirname(fileURLToPath(import.meta.url));

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
    },
  },
  build: {
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (!id.includes("node_modules")) {
            return
          }
          if (id.includes("react-router-dom")) {
            return "vendor-router"
          }
          if (
            id.includes("/react/") ||
            id.includes("/react-dom/")
          ) {
            return "vendor-react"
          }
          if (
            id.includes("@radix-ui/") ||
            id.includes("radix-ui") ||
            id.includes("class-variance-authority") ||
            id.includes("clsx") ||
            id.includes("tailwind-merge")
          ) {
            return "vendor-ui"
          }
          if (id.includes("lucide-react")) {
            return "vendor-icons"
          }
          if (id.includes("qrcode")) {
            return "vendor-qrcode"
          }
          return
        },
      },
    },
  },
  server: {
    port: 5173,
  },
});
