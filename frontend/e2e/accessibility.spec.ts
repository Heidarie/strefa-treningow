import { test, expect } from "@playwright/test";
import AxeBuilder from "@axe-core/playwright";
test("public page and login meet automated WCAG AA checks", async ({
  page,
}) => {
  for (const path of ["/konin/boks", "/konto"]) {
    await page.goto(path);
    const results = await new AxeBuilder({ page })
      .withTags(["wcag2a", "wcag2aa", "wcag21aa"])
      .analyze();
    expect(results.violations).toEqual([]);
  }
});
test("skip link and search work with keyboard", async ({ page }) => {
  await page.goto("/");
  await page.keyboard.press("Tab");
  await expect(
    page.getByRole("link", { name: "Przejdź do treści" }),
  ).toBeFocused();
  await page
    .getByRole("combobox", { name: "Miasto", exact: true })
    .fill("Konin");
  await page
    .getByRole("combobox", { name: "Dyscyplina", exact: true })
    .selectOption("boks");
  await page.getByRole("button", { name: "Szukaj treningu" }).focus();
  await page.keyboard.press("Enter");
  await expect(page).toHaveURL(/\/konin\/boks$/);
});
