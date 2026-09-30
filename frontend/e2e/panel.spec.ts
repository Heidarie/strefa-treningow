import { test, expect } from "@playwright/test";
import { readFileSync } from "node:fs";
const env = Object.fromEntries(
  readFileSync(new URL("../../.env", import.meta.url), "utf8")
    .split("\n")
    .filter((x) => x && !x.startsWith("#") && x.includes("="))
    .map((x) => [x.slice(0, x.indexOf("=")), x.slice(x.indexOf("=") + 1)]),
);
test("club owner manages location, training and schedule through the UI", async ({
  page,
}) => {
  await page.goto("/konto");
  await page.getByLabel("E-mail", { exact: true }).fill(env.ADMIN_EMAIL!);
  await page.getByLabel("Hasło", { exact: true }).fill(env.ADMIN_PASSWORD!);
  await page
    .getByRole("button", { name: "Zaloguj się →", exact: true })
    .click();
  await expect(
    page.getByRole("heading", { name: "Panel organizacji" }),
  ).toBeVisible();
  await page.getByText("Utwórz organizację", { exact: true }).click();
  const name = "UI fixture " + Date.now();
  await page.getByPlaceholder("Nazwa firmy lub klubu").fill(name);
  await page.getByRole("button", { name: "Utwórz", exact: true }).click();
  await expect(
    page.getByText("Organizacja utworzona.", { exact: false }),
  ).toBeVisible();
  await page.getByRole("button", { name: "+ Dodaj lokalizację" }).click();
  await page.getByLabel("Nazwa", { exact: true }).fill(name + " lokalizacja");
  await page.getByLabel("Adres", { exact: true }).fill("Testowa 1");
  await page.getByRole("button", { name: "Zapisz zmiany" }).click();
  await expect(
    page.getByText(name + " lokalizacja", { exact: true }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Treningi", exact: true }).click();
  await page.getByRole("button", { name: "+ Dodaj trening" }).click();
  await page.getByLabel("Nazwa", { exact: true }).fill(name + " pilates");
  await page
    .getByLabel("Opis", { exact: true })
    .fill("Przykładowy opis treningu testowego");
  await page.getByLabel("Cena w złotych (opcjonalnie)").fill("45.50");
  await page.getByRole("button", { name: "Zapisz zmiany" }).click();
  await expect(
    page.getByText(name + " pilates", { exact: true }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Grafik", exact: true }).click();
  await page.getByRole("button", { name: "+ Dodaj do grafiku" }).click();
  await page.getByRole("button", { name: "Zapisz zmiany" }).click();
  await expect(
    page.getByRole("heading", { name: "Serie tygodniowe" }),
  ).toBeVisible();
  await expect(
    page.getByText("Pn, Śr · 18:00 · 60 min", { exact: false }),
  ).toBeVisible();
});
