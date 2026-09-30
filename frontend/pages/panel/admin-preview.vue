<script setup lang="ts">
const { session, call } = useApi(),
  route = useRoute(),
  data = ref<any>(null);
useSeoMeta({ title: "Podgląd moderacji", robots: "noindex,nofollow" });
try {
  await session();
  data.value = await call("/admin/organizations/" + route.query.organization);
} catch {
  await navigateTo("/panel");
}
</script>
<template>
  <div v-if="data" class="content">
    <NuxtLink to="/panel/admin">← Administracja</NuxtLink>
    <h1 style="font-size: 2.5rem; margin-top: 20px">
      {{ data.organizations[0]?.name }}
    </h1>
    <section v-for="l in data.locations" :key="l.id" class="form-card">
      <h2>{{ l.name }}</h2>
      <p>{{ l.address }} · {{ l.city }}</p>
      <article
        v-for="t in data.trainings.filter((t: any) => t.location_id === l.id)"
        :key="t.id"
        class="form-card"
      >
        <h3>{{ t.name }}</h3>
        <p style="white-space: pre-line">{{ t.description }}</p>
        <p>
          Cena: {{ t.price === null ? "Niepodana" : t.price / 100 + " zł" }} ·
          {{ t.category }}
        </p>
        <a
          v-if="t.signup_url"
          :href="t.signup_url"
          target="_blank"
          rel="noopener noreferrer"
          >Link zapisu ↗</a
        >
        <div class="image-picker">
          <img v-for="p in t.photos" :key="p" :src="p" alt="Zdjęcie treningu" />
        </div>
      </article>
    </section>
  </div>
</template>
