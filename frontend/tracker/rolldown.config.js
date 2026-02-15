import { defineConfig } from "rolldown";

export default defineConfig([
  {
    platform: "browser",
    input: "index.js",
    output: {
      format: "iife",
      file: "../../public/hk.js",
      minify: true,
    },
  },
]);
