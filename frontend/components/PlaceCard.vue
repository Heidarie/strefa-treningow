<script setup lang="ts">
import { money, dateLabel } from "~/utils/theme";
defineProps<{ place: any; categories: any[] }>();
</script>
<template>
  <article class="place-card">
    <div class="card-art">
      <img
        v-if="place.trainings[0]?.photos?.[0]"
        :src="place.trainings[0].photos[0]"
        alt=""
        loading="lazy"
        width="400"
        height="92"
      /><span class="tag">{{
        categories.find((c) => c.slug === place.trainings[0]?.category)?.name ||
        "Treningi"
      }}</span>
    </div>
    <div class="card-body">
      <div class="card-top">
        <div>
          <h3>
            <NuxtLink :to="'/lokalizacja/' + place.id">{{
              place.name
            }}</NuxtLink>
          </h3>
          <p class="card-address">
            ⌖ {{ place.address }}
            <span v-if="place.distance_m != null">
              · {{ (place.distance_m / 1000).toFixed(1) }} km</span
            >
          </p>
        </div>
        <img
          v-if="place.logo"
          :src="place.logo"
          class="place-logo"
          alt="Logo klubu"
          width="34"
          height="34"
        />
      </div>
      <div
        v-for="t in place.trainings.slice(0, 3)"
        :key="t.id"
        class="training-mini"
      >
        <NuxtLink :to="'/trening/' + t.id"
          >{{ t.name
          }}<small>{{
            t.next_at ? dateLabel(t.next_at) : "Sprawdź dostępne terminy"
          }}</small></NuxtLink
        ><NuxtLink
          :to="'/trening/' + t.id"
          class="arrow-link"
          :aria-label="'Szczegóły: ' + t.name"
          >↗</NuxtLink
        >
      </div>
      <div class="card-bottom">
        <span>{{ money(place.trainings[0]?.price ?? null) }}</span
        ><span v-if="place.trainings.some((t: any) => t.cards.length)"
          >✓ Karty sportowe</span
        >
      </div>
    </div>
  </article>
</template>
