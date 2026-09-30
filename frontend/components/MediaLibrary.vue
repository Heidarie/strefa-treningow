<script setup lang="ts">
const props = defineProps<{
    organizationId: string;
    items: any[];
    modelValue: string[];
  }>(),
  emit = defineEmits(["update:modelValue", "refresh"]);
const { call } = useApi(),
  busy = ref(false),
  error = ref("");
async function upload(e: Event) {
  const file = (e.target as HTMLInputElement).files?.[0];
  if (!file) return;
  busy.value = true;
  error.value = "";
  try {
    const data = new FormData();
    data.append("organization_id", props.organizationId);
    data.append("file", file);
    await call("/media", { method: "POST", body: data });
    emit("refresh");
  } catch (e: any) {
    error.value = e.message;
  } finally {
    busy.value = false;
  }
}
function toggle(url: string) {
  emit(
    "update:modelValue",
    props.modelValue.includes(url)
      ? props.modelValue.filter((x) => x !== url)
      : [...props.modelValue, url],
  );
}
</script>
<template>
  <div class="form-stack">
    <label
      >Dodaj zdjęcie (JPEG/PNG, do 8 MB)<input
        type="file"
        accept="image/jpeg,image/png"
        :disabled="busy"
        @change="upload"
    /></label>
    <p v-if="error" class="error">{{ error }}</p>
    <p v-if="busy" class="hint">Wysyłanie…</p>
    <div class="image-picker">
      <template
        v-for="m in items.filter((x) => x.organization_id === organizationId)"
        :key="m.id"
        ><button
          v-if="m.status === 'ready'"
          type="button"
          :class="{ selected: modelValue.includes(m.url) }"
          :aria-pressed="modelValue.includes(m.url)"
          aria-label="Wybierz zdjęcie"
          @click="toggle(m.url)"
        >
          <img :src="m.thumbnail" alt="Zdjęcie z biblioteki" /></button
        ><span v-else class="hint">{{
          m.status === "failed"
            ? "Błąd przetwarzania"
            : "Zdjęcie jest przetwarzane…"
        }}</span></template
      >
    </div>
    <p class="hint">
      Kliknij, aby wybrać lub odznaczyć. Pierwsze wybrane zdjęcie jest banerem.
    </p>
  </div>
</template>
