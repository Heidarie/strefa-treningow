<script setup lang="ts">
import * as maplibregl from "maplibre-gl";
import workerUrl from "maplibre-gl/dist/maplibre-gl-worker.mjs?worker&url";
maplibregl.setWorkerUrl(workerUrl);
const props = defineProps<{
  filters: any;
  center: [number, number];
  initialZoom: number;
}>();
const emit = defineEmits<{ viewport: [string] }>();
const target = ref<HTMLDivElement>(),
  message = ref(""),
  missing = !useRuntimeConfig().public.maptilerKey;
let map: maplibregl.Map | undefined,
  controller: AbortController | undefined,
  timer: ReturnType<typeof setTimeout> | undefined;
const key = useRuntimeConfig().public.maptilerKey;
function bounds() {
  if (!map) return "";
  const b = map.getBounds();
  return [
    Math.max(-180, b.getWest()),
    Math.max(-85, b.getSouth()),
    Math.min(180, b.getEast()),
    Math.min(85, b.getNorth()),
  ].join(",");
}
async function load() {
  if (!map?.getSource("places")) return;
  controller?.abort();
  controller = new AbortController();
  const currentRequest = controller;
  try {
    const rows = await $fetch<any[]>("/api/v1/map", {
      query: {
        ...props.filters,
        page: undefined,
        lat: undefined,
        lng: undefined,
        bbox: bounds(),
        zoom: Math.floor(map.getZoom()),
      },
      signal: controller.signal,
    });
    (map.getSource("places") as maplibregl.GeoJSONSource).setData({
      type: "FeatureCollection",
      features: rows.map((p) => ({
        type: "Feature",
        geometry: { type: "Point", coordinates: [p.lng, p.lat] },
        properties: p,
      })),
    });
    message.value = "";
  } catch (e: any) {
    if (!currentRequest.signal.aborted)
      message.value = "Nie udało się odświeżyć mapy. Skorzystaj z listy.";
  }
}
watch(
  target,
  (element) => {
    if (missing || !element || map) return;
    try {
      map = new maplibregl.Map({
        container: element,
        style: `https://api.maptiler.com/maps/streets-v2/style.json?key=${encodeURIComponent(key)}`,
        center: props.center,
        zoom: props.initialZoom,
        renderWorldCopies: false,
        attributionControl: { compact: true },
      });
    } catch {
      message.value =
        "Nie można uruchomić mapy w tej przeglądarce. Skorzystaj z listy.";
      return;
    }
    map.addControl(new maplibregl.NavigationControl(), "bottom-right");
    map.on("error", () => {
      message.value =
        "Mapa jest chwilowo niedostępna. Wyniki znajdziesz na liście.";
    });
    map.on("load", () => {
      map!.addSource("places", {
        type: "geojson",
        data: { type: "FeatureCollection", features: [] },
      });
      map!.addLayer({
        id: "places",
        type: "circle",
        source: "places",
        paint: {
          "circle-radius": ["case", [">", ["get", "count"], 1], 19, 12],
          "circle-color": "#284f38",
          "circle-stroke-width": 3,
          "circle-stroke-color": "#fff",
        },
      });
      map!.addLayer({
        id: "labels",
        type: "symbol",
        source: "places",
        layout: {
          "text-field": [
            "case",
            [">", ["get", "count"], 1],
            ["to-string", ["get", "count"]],
            "",
          ],
          "text-size": 12,
        },
        paint: { "text-color": "#ffffff" },
      });
      load();
      map!.on("click", "places", async (e) => {
        const f = e.features?.[0];
        if (!f || f.geometry.type !== "Point") return;
        const p = f.properties;
        if (Number(p.count) > 1) {
          map!.easeTo({
            center: f.geometry.coordinates as [number, number],
            zoom: map!.getZoom() + 2,
          });
          return;
        }
        const el = document.createElement("div");
        const title = document.createElement("strong");
        title.textContent = p.name;
        el.append(title);
        const link = document.createElement("a");
        link.href = "/lokalizacja/" + encodeURIComponent(p.id);
        link.textContent = "Zobacz treningi →";
        el.append(link);
        new maplibregl.Popup()
          .setLngLat(f.geometry.coordinates as [number, number])
          .setDOMContent(el)
          .addTo(map!);
        const summary = document.createElement("div");
        summary.textContent = "Ładowanie najbliższych treningów…";
        el.insertBefore(summary, link);
        try {
          const location = await $fetch<any>(
            "/api/v1/locations/" + encodeURIComponent(p.id),
          );
          summary.replaceChildren();
          const organization = document.createElement("p");
          organization.textContent = location.organization;
          summary.append(organization);
          for (const training of location.trainings.slice(0, 3)) {
            const item = document.createElement("p");
            const trainingLink = document.createElement("a");
            trainingLink.href = "/trening/" + encodeURIComponent(training.id);
            trainingLink.textContent = training.name;
            item.append(trainingLink);
            if (training.next_at) {
              const time = document.createElement("time");
              time.dateTime = training.next_at;
              time.textContent = new Intl.DateTimeFormat("pl-PL", {
                timeZone: "Europe/Warsaw",
                dateStyle: "short",
                timeStyle: "short",
              }).format(new Date(training.next_at));
              item.append(document.createElement("br"), time);
            }
            summary.append(item);
          }
        } catch {
          summary.textContent = "Otwórz lokalizację, aby sprawdzić grafik.";
        }
      });
      map!.on(
        "mouseenter",
        "places",
        () => (map!.getCanvas().style.cursor = "pointer"),
      );
      map!.on(
        "mouseleave",
        "places",
        () => (map!.getCanvas().style.cursor = ""),
      );
    });
    map.on("moveend", () => {
      clearTimeout(timer);
      timer = setTimeout(() => {
        emit("viewport", bounds());
        load();
      }, 250);
    });
  },
  { flush: "post" },
);
watch(
  () => [
    props.filters.city,
    props.filters.category,
    props.filters.day,
    props.filters.card,
  ],
  () => load(),
);
watch(
  [() => props.center[0], () => props.center[1], () => props.initialZoom],
  () => map?.easeTo({ center: props.center, zoom: props.initialZoom }),
);
onBeforeUnmount(() => {
  clearTimeout(timer);
  controller?.abort();
  map?.remove();
});
</script>
<template>
  <div v-if="missing" class="map-empty">
    <span class="map-symbol">⌖</span
    ><strong>Wszystkie miejsca są na liście</strong>
    <p>Mapa będzie dostępna po skonfigurowaniu usługi mapowej.</p>
  </div>
  <template v-else
    ><p v-if="message" class="map-status" role="status">{{ message }}</p>
    <div
      ref="target"
      class="map-canvas"
      aria-label="Mapa lokalizacji sportowych"
  /></template>
</template>
