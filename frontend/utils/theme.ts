export function palette(seed: string) {
  let hash = 2166136261;
  for (const c of seed) {
    hash ^= c.charCodeAt(0);
    hash = Math.imul(hash, 16777619);
  }
  const hue = (hash >>> 0) % 360;
  // Low lightness guarantees readable white text across all hues.
  return {
    "--accent": `hsl(${hue} 62% 20%)`,
    "--accent-soft": `hsl(${hue} 40% 93%)`,
    "--accent-bright": `hsl(${hue} 64% 78%)`,
  };
}
export function normalize(value: string) {
  return value
    .toLocaleLowerCase("pl")
    .normalize("NFD")
    .replace(/[\u0300-\u036f]/g, "")
    .replaceAll("ł", "l")
    .trim();
}
export function money(cents: number | null) {
  return cents === null
    ? "Cena niepodana"
    : cents === 0
      ? "Bezpłatnie"
      : new Intl.NumberFormat("pl-PL", {
          style: "currency",
          currency: "PLN",
          maximumFractionDigits: 2,
        }).format(cents / 100);
}
export function dateLabel(date: string) {
  return new Intl.DateTimeFormat("pl-PL", {
    weekday: "short",
    day: "numeric",
    month: "short",
    hour: "2-digit",
    minute: "2-digit",
    timeZone: "Europe/Warsaw",
  }).format(new Date(date));
}

export function warsawDate(date = new Date()) {
  return new Intl.DateTimeFormat("sv-SE", {
    timeZone: "Europe/Warsaw",
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  }).format(date);
}
