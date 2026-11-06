import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";
export default defineConfig({
  plugins: [vue()],
  build: {
    outDir: "../internal/dashboardassets/dist",
    emptyOutDir: true,
    sourcemap: false,
    target: "es2022",
  },
});
