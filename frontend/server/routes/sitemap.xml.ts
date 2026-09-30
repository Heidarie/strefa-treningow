export default defineEventHandler(async (event) => {
  const config = useRuntimeConfig(event);
  const paths = await $fetch<{ path: string }[]>(
    config.apiBase + "/api/v1/sitemap",
  );
  const escape = (v: string) =>
    v
      .replaceAll("&", "&amp;")
      .replaceAll("<", "&lt;")
      .replaceAll('"', "&quot;");
  setHeader(event, "Content-Type", "application/xml");
  return (
    '<?xml version="1.0" encoding="UTF-8"?><urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">' +
    paths
      .map(
        (p) =>
          `<url><loc>${escape(config.public.siteUrl + p.path)}</loc></url>`,
      )
      .join("") +
    "</urlset>"
  );
});
