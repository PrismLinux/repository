import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";

// https://vitejs.dev/config/
export default defineConfig({
  base: "/linux/CrystalLinux/tooling/package-repository/",
  plugins: [vue()],
});
