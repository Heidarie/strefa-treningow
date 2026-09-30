export default defineEventHandler(async (event) => {
  const path = getRouterParam(event, "path") || "";
  if (!/^[0-9a-f-]{36}(-thumb)?\.jpg$/.test(path))
    throw createError({ statusCode: 404 });
  setHeader(event, "Cache-Control", "private,no-store");
  // Public, processed images only. Original uploads are never exposed.
  return proxyRequest(
    event,
    useRuntimeConfig(event).apiBase + "/public-media/" + path,
  );
});
