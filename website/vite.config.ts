import { defineConfig } from "vite";
import vue from "@vitejs/plugin-vue";

// https://vitejs.dev/config/
export default defineConfig({
  base: "/linux/prismlinux/websites/repository",
  plugins: [vue()],
});
