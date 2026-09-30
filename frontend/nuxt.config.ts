export default defineNuxtConfig({
  srcDir: ".",
  compatibilityDate: "2025-09-01",
  devtools: { enabled: false },
  css: ["~/assets/main.css", "maplibre-gl/dist/maplibre-gl.css"],
  runtimeConfig: {
    apiBase: "http://localhost:8080",
    public: {
      siteUrl: "http://localhost:3000",
      maptilerKey: "",
      themeSeed: "strefa-7e13baf4",
    },
  },
  app: {
    head: {
      htmlAttrs: { lang: "pl" },
      title: "Strefa Treningów",
      meta: [{ name: "theme-color", content: "#f7f7f2" }],
    },
  },
  routeRules: {
    "/panel/**": { headers: { "X-Robots-Tag": "noindex" } },
    "/konto": { headers: { "X-Robots-Tag": "noindex" } },
  },
  typescript: { strict: true },
});
