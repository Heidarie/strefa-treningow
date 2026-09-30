<script setup lang="ts">
import { money, dateLabel } from "~/utils/theme";
const route = useRoute(),
  config = useRuntimeConfig(),
  { call } = useApi();
const { data: t, error } = await useAsyncData(
  "training:" + route.params.id,
  () => call("/trainings/" + route.params.id),
);
if (error.value || !t.value)
  throw createError({
    statusCode: error.value?.statusCode || 404,
    statusMessage:
      error.value?.statusCode === 404
        ? "Nie znaleziono treningu"
        : "Strona jest chwilowo niedostępna",
  });
const { data: dict } = await useAsyncData("dictionaries", () =>
  call("/dictionaries"),
);
useSeoMeta({
  title: () => `${t.value.name} — ${t.value.organization} | Strefa Treningów`,
  description: () => t.value.description.slice(0, 160),
  ogImage: () =>
    t.value.photos[0] ? config.public.siteUrl + t.value.photos[0] : undefined,
});
useHead(() => ({
  link: [{ rel: "canonical", href: config.public.siteUrl + route.path }],
  script: [
    {
      type: "application/ld+json",
      innerHTML: JSON.stringify({
        "@context": "https://schema.org",
        "@type": "Service",
        name: t.value.name,
        description: t.value.description,
        provider: {
          "@type": "SportsActivityLocation",
          name: t.value.organization,
          address: t.value.location.address,
        },
        ...(t.value.price !== null
          ? {
              offers: {
                "@type": "Offer",
                price: t.value.price / 100,
                priceCurrency: "PLN",
              },
            }
          : {}),
      }).replaceAll("<", "\\u003c"),
    },
  ],
}));
</script>
<template>
  <div class="content">
    <nav class="breadcrumbs" aria-label="Ścieżka">
      <NuxtLink to="/">Odkrywaj</NuxtLink><span>/</span
      ><NuxtLink :to="'/' + t.location.city">{{
        dict?.cities.find((c: any) => c.slug === t.location.city)?.name
      }}</NuxtLink
      ><span>/</span
      ><NuxtLink :to="'/lokalizacja/' + t.location.id">{{
        t.location.name
      }}</NuxtLink>
    </nav>
    <div class="detail-hero">
      <img
        v-if="t.photos[0]"
        :src="t.photos[0]"
        :alt="t.name"
        width="1120"
        height="280"
      /><span v-else aria-hidden="true">◒</span>
    </div>
    <div class="detail-layout">
      <div>
        <span class="eyebrow">{{
          dict?.categories.find((c: any) => c.slug === t.category)?.name
        }}</span>
        <h1>{{ t.name }}</h1>
        <p class="muted">{{ t.organization }} · {{ t.location.address }}</p>
        <section>
          <h2>O treningu</h2>
          <p>
            {{ t.description || "Organizator nie dodał jeszcze opisu zajęć." }}
          </p>
        </section>
        <section>
          <h2>Najbliższe terminy</h2>
          <div v-for="o in t.occurrences" :key="o.id" class="schedule-item">
            <span>{{ dateLabel(o.starts_at) }}</span
            ><span class="muted"
              >{{
                Math.round(
                  (+new Date(o.ends_at) - +new Date(o.starts_at)) / 60000,
                )
              }}
              min</span
            >
          </div>
          <p v-if="!t.occurrences.length" class="muted">
            Organizator nie opublikował jeszcze kolejnych terminów.
          </p>
        </section>
        <section v-if="t.photos.length > 1" class="image-picker">
          <img
            v-for="p in t.photos.slice(1)"
            :key="p"
            :src="p"
            :alt="t.name"
            loading="lazy"
            style="max-width: 100%; border-radius: 12px"
          />
        </section>
      </div>
      <aside class="booking">
        <span class="eyebrow">ZNAJDŹ CZAS DLA SIEBIE</span>
        <div class="price">{{ money(t.price) }}</div>
        <p class="muted">{{ t.location.name }}<br />{{ t.location.address }}</p>
        <a
          v-if="t.signup_url"
          class="button"
          :href="t.signup_url"
          rel="noopener noreferrer"
          >Zapisz się ↗</a
        >
        <p v-if="t.signup_url" class="hint">
          Zapisy odbywają się na stronie organizatora.
        </p>
        <p v-else class="notice">Organizator nie udostępnił zapisów online.</p>
        <div v-if="t.cards.length" style="margin-top: 20px">
          <p class="hint">Akceptowane karty</p>
          <div class="inline" style="margin-top: 8px">
            <span v-for="c in t.cards" :key="c" class="badge">{{
              dict?.cards.find((x: any) => x.slug === c)?.name || c
            }}</span>
          </div>
        </div>
        <NuxtLink
          class="button secondary"
          :to="'/lokalizacja/' + t.location.id"
          style="margin-top: 20px"
          >Zobacz lokalizację</NuxtLink
        >
      </aside>
    </div>
  </div>
</template>
