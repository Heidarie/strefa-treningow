import { test, expect } from "@playwright/test";
test.skip(
  !process.env.MAP_TEST_URL,
  "Requires a separate frontend with a test map key",
);
test("map integration: rendered marker opens the matching location", async ({
  page,
  request,
}) => {
  const response = await request.get(
    "http://localhost:18080/api/v1/search?city=konin",
  );
  const locations = await response.json();
  const location = locations.find((l: any) => l.organization.includes("DEMO"));
  await page.route("https://api.maptiler.com/**", (route) =>
    route.fulfill({
      json: {
        version: 8,
        sources: {},
        glyphs: "https://api.maptiler.com/fonts/{fontstack}/{range}.pbf",
        layers: [
          {
            id: "background",
            type: "background",
            paint: { "background-color": "#e9eee2" },
          },
        ],
      },
    }),
  );
  const mapData = page.waitForResponse(
    (r) => r.url().includes("/api/v1/map") && r.status() === 200,
  );
  await page.goto(process.env.MAP_TEST_URL + "/lokalizacja/" + location.id);
  await mapData;
  const canvas = page.locator("canvas.maplibregl-canvas");
  await expect(canvas).toBeVisible();
  // GeoJSON is rendered asynchronously by the map worker after the API response.
  await expect(async () => {
    await canvas.click();
    await expect(page.locator(".maplibregl-popup")).toContainText(
      location.name,
      { timeout: 500 },
    );
  }).toPass({ timeout: 10000 });
  await expect(page.locator(".maplibregl-popup")).toContainText(
    "Boks od podstaw",
  );
  await expect(page.locator(".maplibregl-popup time")).toBeVisible();
});
test("map integration: provider failure leaves public list usable", async ({
  page,
}) => {
  await page.route("https://api.maptiler.com/**", (route) =>
    route.fulfill({ status: 503, body: "unavailable" }),
  );
  await page.goto(process.env.MAP_TEST_URL + "/konin/boks");
  await expect(page.getByRole("status")).toContainText(
    "Mapa jest chwilowo niedostępna",
  );
  await expect(
    page.getByRole("link", { name: "Boks od podstaw", exact: false }).first(),
  ).toBeVisible();
});
