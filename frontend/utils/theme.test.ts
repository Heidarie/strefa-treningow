import { test } from "node:test";
import assert from "node:assert/strict";
import { palette, normalize, money, warsawDate } from "./theme";
test("seed is deterministic", () =>
  assert.deepEqual(palette("seed"), palette("seed")));
test("Polish aliases", () => assert.equal(normalize("ŁÓDŹ"), "lodz"));
test("missing price does not mean free", () => {
  assert.equal(money(null), "Cena niepodana");
  assert.equal(money(0), "Bezpłatnie");
});
test("white text meets WCAG AA for every generated hue", () => {
  for (let h = 0; h < 360; h++) {
    const s = 0.62,
      l = 0.2,
      a = s * Math.min(l, 1 - l);
    const channel = (n: number) => {
      const k = (n + h / 30) % 12;
      const c = l - a * Math.max(-1, Math.min(k - 3, 9 - k, 1));
      return c <= 0.04045 ? c / 12.92 : ((c + 0.055) / 1.055) ** 2.4;
    };
    const luminance =
      0.2126 * channel(0) + 0.7152 * channel(8) + 0.0722 * channel(4);
    assert.ok(1.05 / (luminance + 0.05) >= 4.5);
  }
});

test("dates use Polish local day", () =>
  assert.equal(warsawDate(new Date("2026-09-28T22:30:00Z")), "2026-09-29"));
