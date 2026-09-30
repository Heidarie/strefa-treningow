<script setup lang="ts">
// SSR stays readable; enable JS controls only after their handlers are mounted.
const ready = ref(false);
onMounted(() => {
  ready.value = true;
});
import { normalize } from "~/utils/theme";
const route = useRoute(),
  router = useRouter(),
  config = useRuntimeConfig();
const { call, catalog } = useApi();
const { data: dict, error: dictError } = await useAsyncData(
  "dictionaries",
  () => catalog(),
);
const city = computed(() => String(route.params.city || ""));
const category = computed(() => String(route.params.category || ""));
if (
  dict.value &&
  ((city.value && !dict.value.cities.some((c: any) => c.slug === city.value)) ||
    (category.value &&
      !dict.value.categories.some((c: any) => c.slug === category.value)))
)
  throw createError({
    statusCode: 404,
    statusMessage: "Nie znaleziono strony",
  });
const cityData = computed(() =>
  dict.value?.cities.find((c: any) => c.slug === city.value),
);
const categoryData = computed(() =>
  dict.value?.categories.find((c: any) => c.slug === category.value),
);
const cityInput = ref(cityData.value?.name || "");
const selectedCategory = ref(category.value);
const day = ref(String(route.query.day || "")),
  card = ref(String(route.query.card || ""));
const mode = ref(route.query.view === "list" ? "list" : "map");
const coords = ref<{ lat: number; lng: number } | null>(null),
  geoMessage = ref(""),
  cityMessage = ref("");
const bbox = ref("");
const filters = computed(() => ({
  city: city.value || undefined,
  category: category.value || undefined,
  day: route.query.day || undefined,
  card: route.query.card || undefined,
  bbox: bbox.value || undefined,
  lat: coords.value?.lat,
  lng: coords.value?.lng,
  page: route.query.page || 1,
}));
const {
  data: results,
  status,
  error,
  refresh,
} = await useAsyncData(
  "search:" + route.path,
  (_nuxt, { signal }) => call("/search", { query: filters.value, signal }),
  { watch: [filters] },
);
if (error.value || dictError.value)
  throw createError({
    statusCode: 503,
    statusMessage: "Wyszukiwarka jest chwilowo niedostępna",
  });
const total = computed(() => results.value?.[0]?.total || 0);
const heading = computed(() =>
  categoryData.value
    ? `${categoryData.value.name} ${cityData.value?.name || ""}`
    : cityData.value
      ? `Treningi ${cityData.value.name}`
      : "Znajdź swoją strefę.",
);
const title = computed(() =>
  city.value
    ? heading.value + " — kluby i grafik | Strefa Treningów"
    : "Strefa Treningów — odkryj treningi w swojej okolicy",
);
useSeoMeta({
  title,
  description: () =>
    `Odkryj ${categoryData.value?.name?.toLowerCase() || "treningi"}${cityData.value ? " w mieście " + cityData.value.name : " w swojej okolicy"}. Porównaj kluby, sprawdź grafik i znajdź zajęcia dla siebie.`,
  robots: () =>
    Object.keys(route.query).length || (city.value && !total.value)
      ? "noindex,follow"
      : "index,follow",
});
useHead(() => ({
  link: [{ rel: "canonical", href: config.public.siteUrl + route.path }],
  script: [
    {
      type: "application/ld+json",
      innerHTML: JSON.stringify({
        "@context": "https://schema.org",
        "@type": "BreadcrumbList",
        itemListElement: [
          {
            "@type": "ListItem",
            position: 1,
            name: "Strefa Treningów",
            item: config.public.siteUrl,
          },
          ...(cityData.value
            ? [
                {
                  "@type": "ListItem",
                  position: 2,
                  name: cityData.value.name,
                  item: config.public.siteUrl + "/" + city.value,
                },
              ]
            : []),
          ...(categoryData.value
            ? [
                {
                  "@type": "ListItem",
                  position: 3,
                  name: categoryData.value.name,
                  item: config.public.siteUrl + route.path,
                },
              ]
            : []),
        ],
      }).replaceAll("<", "\\u003c"),
    },
  ],
}));
const symbols: Record<string, string> = {
  pilates: "◒",
  joga: "✳",
  boks: "◈",
  silownia: "▥",
  crossfit: "↗",
  taniec: "♫",
  plywanie: "≈",
  "sztuki-walki": "✦",
};
async function search(cat?: string) {
  cityMessage.value = "";
  const input = normalize(cityInput.value);
  const matches =
    dict.value?.cities.filter(
      (c: any) =>
        normalize(c.name) === input ||
        normalize(c.slug) === input ||
        normalize(`${c.name} (${c.region})`) === input ||
        c.aliases.some((a: string) => normalize(a) === input),
    ) || [];
  if (input && matches.length !== 1) {
    cityMessage.value =
      matches.length > 1
        ? "Wybierz miasto z województwem z podpowiedzi."
        : "Wybierz miasto z podpowiedzi.";
    return;
  }
  const c = matches[0]?.slug || "";
  const k = cat === undefined ? selectedCategory.value : cat;
  if (k && !c) {
    cityMessage.value = "Wybierz miasto, aby wyszukać tę dyscyplinę.";
    return;
  }
  bbox.value = "";
  await router.push({
    path: c ? "/" + c + (k ? "/" + k : "") : "/",
    query: {
      ...(day.value ? { day: day.value } : {}),
      ...(card.value ? { card: card.value } : {}),
    },
  });
}
function locate() {
  geoMessage.value = "Ustalam lokalizację…";
  if (!navigator.geolocation) {
    geoMessage.value = "Twoja przeglądarka nie udostępnia lokalizacji.";
    return;
  }
  navigator.geolocation.getCurrentPosition(
    (p) => {
      coords.value = { lat: p.coords.latitude, lng: p.coords.longitude };
      geoMessage.value = "Odległości w linii prostej od Twojej lokalizacji.";
    },
    () => {
      geoMessage.value =
        "Nie uzyskaliśmy lokalizacji. Nadal możesz wybrać miasto.";
    },
    { timeout: 10000, maximumAge: 300000 },
  );
}
function viewport(value: string) {
  bbox.value = value;
  if (route.query.page)
    router.replace({ query: { ...route.query, page: undefined } });
}
watch(
  () => route.path,
  () => {
    cityInput.value = cityData.value?.name || "";
    selectedCategory.value = category.value;
    bbox.value = "";
  },
);
</script>
<template>
  <section class="hero">
    <div>
      <div class="eyebrow">
        <span class="live-dot" /> TWOJE MIASTO. TWÓJ RUCH.
      </div>
      <h1 v-if="!city">
        Znajdź swoją strefę.<br /><em>Zrób pierwszy ruch.</em>
      </h1>
      <h1 v-else>{{ heading }}<br /><em>W Twoim rytmie.</em></h1>
      <p>
        Dobry trening zaczyna się blisko Ciebie. Odkrywaj miejsca, sprawdzaj
        grafik i wybierz coś dla siebie.
      </p>
    </div>
    <div class="hero-aside">
      <div class="orbit">
        <span>↗</span><span>◒</span><span>✳</span><span>◈</span>
      </div>
      <p>
        Od pierwszych kroków<br />po nowe rekordy.
        <strong>Jest tu miejsce dla Ciebie.</strong>
      </p>
    </div>
  </section>
  <section class="explorer" aria-label="Wyszukiwarka treningów">
    <form class="search-bar" @submit.prevent="search()">
      <label
        ><span class="sr-only">Miasto</span
        ><input
          v-model="cityInput"
          list="cities"
          placeholder="⌖  W jakim mieście?"
          aria-label="Miasto"
          :disabled="!ready" /><datalist id="cities">
          <option
            v-for="c in dict?.cities"
            :key="c.slug"
            :value="`${c.name} (${c.region})`"
          /></datalist></label
      ><label
        ><span class="sr-only">Dyscyplina</span
        ><select
          v-model="selectedCategory"
          aria-label="Dyscyplina"
          :disabled="!ready"
        >
          <option value="">Wszystkie aktywności</option>
          <option v-for="c in dict?.categories" :key="c.slug" :value="c.slug">
            {{ c.name }}
          </option>
        </select></label
      ><label class="optional-filter"
        ><select v-model="day" aria-label="Dzień tygodnia" :disabled="!ready">
          <option value="">Dowolny dzień</option>
          <option
            v-for="(d, i) in [
              'Niedziela',
              'Poniedziałek',
              'Wtorek',
              'Środa',
              'Czwartek',
              'Piątek',
              'Sobota',
            ]"
            :key="i"
            :value="String(i)"
          >
            {{ d }}
          </option>
        </select></label
      ><label class="optional-filter"
        ><select v-model="card" aria-label="Karta sportowa" :disabled="!ready">
          <option value="">Wszystkie karty</option>
          <option v-for="c in dict?.cards" :key="c.slug" :value="c.slug">
            {{ c.name }}
          </option>
        </select></label
      ><button :disabled="!ready">Szukaj treningu <span>↗</span></button>
    </form>
    <p v-if="cityMessage" class="error" role="alert">{{ cityMessage }}</p>
    <div class="chips" aria-label="Kategorie">
      <button
        class="chip"
        :class="{ active: !category }"
        @click="search('')"
        :disabled="!ready"
      >
        <span class="symbol">⊞</span>Wszystko</button
      ><button
        v-for="c in dict?.categories"
        :key="c.slug"
        class="chip"
        :class="{ active: category === c.slug }"
        @click="search(c.slug)"
        :disabled="!ready"
      >
        <span class="symbol">{{ symbols[c.slug] || "◇" }}</span
        >{{ c.name }}
      </button>
    </div>
    <div class="explore-toolbar">
      <div>
        <span class="label">{{ cityData?.name || "Odkrywaj Polskę" }}</span
        ><span class="count">{{ total }} miejsc dla Ciebie</span>
      </div>
      <div class="toolbar-actions">
        <button class="locate" @click="locate" :disabled="!ready">
          ⌖ Użyj mojej lokalizacji
        </button>
        <div class="view-toggle" aria-label="Widok wyników">
          <button
            :class="{ active: mode === 'map' }"
            :aria-pressed="mode === 'map'"
            @click="mode = 'map'"
            :disabled="!ready"
          >
            ⊞ Mapa</button
          ><button
            :class="{ active: mode === 'list' }"
            :aria-pressed="mode === 'list'"
            @click="mode = 'list'"
            :disabled="!ready"
          >
            ☷ Lista
          </button>
        </div>
      </div>
    </div>
    <p v-if="geoMessage" class="notice" role="status">{{ geoMessage }}</p>
    <div v-if="error || dictError" class="error" role="alert">
      Nie udało się pobrać wyników.
      <button class="ghost" @click="refresh()" :disabled="!ready">
        Spróbuj ponownie
      </button>
    </div>
    <div
      :class="{ 'results-layout': mode === 'map' }"
      :aria-busy="status === 'pending'"
    >
      <div
        class="result-list"
        :class="{ full: mode === 'list', pending: status === 'pending' }"
      >
        <PlaceCard
          v-for="place in results"
          :key="place.id"
          :place="place"
          :categories="dict?.categories || []"
        />
        <div v-if="!results?.length && !error" class="empty">
          <h3>Tu zaczyna się coś nowego.</h3>
          <p>
            Nie ma jeszcze zajęć dla wybranych filtrów. Sprawdź inne miasto lub
            aktywność.
          </p>
          <NuxtLink to="/" class="button secondary"
            >Pokaż wszystkie miejsca</NuxtLink
          >
        </div>
      </div>
      <div v-if="mode === 'map'" class="map-container">
        <ClientOnly
          ><TrainingMap
            :filters="filters"
            :center="cityData ? [cityData.lng, cityData.lat] : [19.1, 52.1]"
            :initial-zoom="cityData ? 12 : 6"
            @viewport="viewport"
          /><template #fallback
            ><div class="map-empty">
              <span class="map-symbol">⌖</span
              ><strong>Twoja okolica, nowe możliwości</strong>
              <p>Ładujemy mapę. Wszystkie wyniki są też dostępne na liście.</p>
            </div></template
          ></ClientOnly
        >
      </div>
    </div>
    <div class="pagination">
      <NuxtLink
        v-if="Number(route.query.page || 1) > 1"
        :to="{
          path: route.path,
          query: { ...route.query, page: Number(route.query.page) - 1 },
        }"
        >← Poprzednia</NuxtLink
      ><span class="hint">{{ results?.length || 0 }} z {{ total }} miejsc</span
      ><NuxtLink
        v-if="Number(route.query.page || 1) * 20 < total"
        :to="{
          path: route.path,
          query: { ...route.query, page: Number(route.query.page || 1) + 1 },
        }"
        >Następna →</NuxtLink
      >
    </div>
    <div class="below-explorer">
      <div>
        <h3>Tworzysz miejsce, w którym chce się trenować?</h3>
        <p>Dodaj swój klub i daj się znaleźć osobom z okolicy.</p>
      </div>
      <NuxtLink class="button secondary" to="/panel"
        >Dołącz ze swoim klubem ↗</NuxtLink
      >
    </div>
    <div class="inline" style="margin-top: 24px">
      <NuxtLink
        v-for="c in dict?.cities"
        :key="c.slug"
        :to="'/' + c.slug"
        class="hint"
        >Treningi {{ c.name }}</NuxtLink
      >
    </div>
  </section>
</template>
