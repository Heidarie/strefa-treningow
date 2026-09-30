export default defineEventHandler((event) => {
  setHeader(event, "Content-Type", "text/plain");
  return `User-agent: *\nDisallow: /api/\nSitemap: ${useRuntimeConfig(event).public.siteUrl}/sitemap.xml\n`;
});
