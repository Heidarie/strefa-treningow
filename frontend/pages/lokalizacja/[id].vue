<script setup lang="ts">
const route = useRoute(),
  config = useRuntimeConfig(),
  { call } = useApi();
const { data: l, error } = await useAsyncData(
  "location:" + route.params.id,
  () => call("/locations/" + route.params.id),
);
if (error.value || !l.value)
  throw createError({
    statusCode: error.value?.statusCode || 404,
    statusMessage:
      error.value?.statusCode === 404
        ? "Nie znaleziono lokalizacji"
        : "Strona jest chwilowo niedostępna",
  });
const { data: dict } = await useAsyncData("dictionaries", () =>
  call("/dictionaries"),
);
useSeoMeta({
  title: () => `${l.value.name} — treningi i grafik | Strefa Treningów`,
  description: () =>
    `${l.value.name}, ${l.value.address}. Sprawdź dostępne treningi i najbliższe terminy.`,
});
useHead(() => ({
  link: [{ rel: "canonical", href: config.public.siteUrl + route.path }],
  script: [
    {
      type: "application/ld+json",
      innerHTML: JSON.stringify({
        "@context": "https://schema.org",
        "@type": "SportsActivityLocation",
        name: l.value.name,
        address: l.value.address,
        geo: {
          "@type": "GeoCoordinates",
          latitude: l.value.lat,
          longitude: l.value.lng,
        },
      }).replaceAll("<", "\\u003c"),
    },
  ],
}));
</script>
<template>
  <div class="content">
    <nav class="breadcrumbs" aria-label="Ścieżka">
      <NuxtLink to="/">Odkrywaj</NuxtLink><span>/</span
      ><NuxtLink :to="'/' + l.city">{{
        dict?.cities.find((c: any) => c.slug === l.city)?.name
      }}</NuxtLink>
    </nav>
    <div class="inline">
      <img
        v-if="l.logo"
        :src="l.logo"
        alt="Logo lokalizacji"
        width="70"
        height="70"
      />
      <h1 style="font-size: 2.8rem">{{ l.name }}</h1>
    </div>
    <p class="muted" style="margin: 18px 0 28px">
      ⌖ {{ l.address }} · {{ l.organization }}
    </p>
    <h2 style="margin-bottom: 20px">Wybierz swój trening</h2>
    <div class="result-list full">
      <PlaceCard
        v-for="t in l.trainings"
        :key="t.id"
        :place="{ ...l, trainings: [t] }"
        :categories="dict?.categories || []"
      />
    </div>
    <div
      class="map-container"
      style="position: relative; height: 360px; margin-top: 28px"
    >
      <ClientOnly
        ><TrainingMap
          :filters="{ city: l.city }"
          :center="[l.lng, l.lat]"
          :initial-zoom="15"
      /></ClientOnly>
    </div>
  </div>
</template>
