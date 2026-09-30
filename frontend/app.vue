<script setup lang="ts">
import { palette } from "~/utils/theme";
const theme = palette(useRuntimeConfig().public.themeSeed);
</script>
<template>
  <div :style="theme" class="app">
    <a class="skip" href="#main">Przejdź do treści</a>
    <header class="site-header">
      <NuxtLink
        class="brand"
        to="/"
        aria-label="Strefa Treningów — strona główna"
        ><span class="brand-icon">s<span>t</span></span
        ><span>strefa<span class="brand-light">treningów</span></span></NuxtLink
      >
      <nav aria-label="Nawigacja główna">
        <NuxtLink to="/">Odkrywaj treningi</NuxtLink
        ><NuxtLink to="/panel" class="club-link"
          >Dla klubów <span>↗</span></NuxtLink
        >
      </nav>
    </header>
    <main id="main" tabindex="-1"><NuxtPage /></main>
    <footer>
      <NuxtLink class="brand" to="/">strefa treningów</NuxtLink
      ><span>Twój ruch. Twoje miejsce.</span
      ><span>© {{ new Date().getFullYear() }} Strefa Treningów</span>
    </footer>
  </div>
</template>
