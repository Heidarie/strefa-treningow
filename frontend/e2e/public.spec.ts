import { test, expect } from "@playwright/test";
test("public discovery, details and no map key fallback", async ({ page }) => {
  const errors: string[] = [];
  page.on("pageerror", (e) => errors.push(e.message));
  await page.goto("/konin/boks");
  await expect(page.getByRole("heading", { name: /Boks Konin/ })).toBeVisible();
  await expect(
    page.getByRole("link", { name: "Boks od podstaw", exact: false }).first(),
  ).toBeVisible();
  await page.getByRole("button", { name: "☷ Lista" }).click();
  await expect(page.locator(".result-list.full")).toBeVisible();
  await page
    .getByRole("link", { name: "Boks od podstaw", exact: false })
    .first()
    .click();
  await expect(
    page.getByRole("heading", { name: "Boks od podstaw", exact: true }),
  ).toBeVisible();
  await expect(
    page.getByRole("heading", { name: "Najbliższe terminy" }),
  ).toBeVisible();
  expect(errors).toEqual([]);
});
test("SSR works without JavaScript", async ({ browser }) => {
  const context = await browser.newContext({ javaScriptEnabled: false });
  const page = await context.newPage();
  await page.goto("http://localhost:3100/konin/boks");
  await expect(page.getByRole("heading", { name: /Boks Konin/ })).toBeVisible();
  await expect(
    page.getByRole("link", { name: "Boks od podstaw", exact: false }).first(),
  ).toBeVisible();
  await context.close();
});
test("geolocation denial keeps city and results", async ({ page }) => {
  await page.addInitScript(() =>
    Object.defineProperty(navigator, "geolocation", {
      value: { getCurrentPosition: (_ok: any, fail: any) => fail({ code: 1 }) },
    }),
  );
  await page.goto("/konin/boks");
  await page.getByRole("button", { name: "Użyj mojej lokalizacji" }).click();
  await expect(page.getByRole("status")).toContainText(
    "Nie uzyskaliśmy lokalizacji",
  );
  await expect(page.getByRole("heading", { name: /Boks Konin/ })).toBeVisible();
});
test("no horizontal overflow and screenshot", async ({ page }, info) => {
  await page.goto("/");
  await page.evaluate(() => document.fonts.ready);
  expect(
    await page.evaluate(
      () => document.documentElement.scrollWidth <= window.innerWidth,
    ),
  ).toBe(true);
  await page.screenshot({
    path: `/tmp/strefa-${info.project.name}.png`,
    fullPage: true,
  });
});
