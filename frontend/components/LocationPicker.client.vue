<script setup lang="ts">
import * as maplibregl from "maplibre-gl";
import workerUrl from "maplibre-gl/dist/maplibre-gl-worker.mjs?worker&url";
maplibregl.setWorkerUrl(workerUrl);
const props = defineProps<{ lat: number; lng: number }>(),
  emit = defineEmits<{ change: [{ lat: number; lng: number }] }>();
const message = ref("");
const el = ref<HTMLDivElement>(),
  key = useRuntimeConfig().public.maptilerKey;
let map: maplibregl.Map | undefined, marker: maplibregl.Marker | undefined;
watch(
  el,
  (element) => {
    if (!key || !element || map) return;
    try {
      map = new maplibregl.Map({
        container: element,
        style: `https://api.maptiler.com/maps/streets-v2/style.json?key=${encodeURIComponent(key)}`,
        center: [props.lng, props.lat],
        zoom: 13,
        renderWorldCopies: false,
      });
    } catch {
      message.value = "Mapa jest niedostępna. Podaj współrzędne ręcznie.";
      return;
    }
    map.on("error", () => {
      message.value = "Mapa jest niedostępna. Podaj współrzędne ręcznie.";
    });
    marker = new maplibregl.Marker({ draggable: true, color: "#284f38" })
      .setLngLat([props.lng, props.lat])
      .addTo(map);
    marker.on("dragend", () => emit("change", marker!.getLngLat()));
    map.on("click", (e) => {
      marker!.setLngLat(e.lngLat);
      emit("change", e.lngLat);
    });
  },
  { flush: "post" },
);
watch(
  () => [props.lng, props.lat],
  () => {
    marker?.setLngLat([props.lng, props.lat]);
    map?.easeTo({ center: [props.lng, props.lat] });
  },
);
onBeforeUnmount(() => map?.remove());
</script>
<template>
  <p v-if="message" role="status" class="hint">{{ message }}</p>
  <div
    v-if="key"
    ref="el"
    style="height: 250px; border-radius: 10px; overflow: hidden"
    aria-label="Potwierdź pozycję lokalizacji na mapie"
  />
  <p v-else class="hint">Brak klucza map — możesz podać współrzędne ręcznie.</p>
</template>
