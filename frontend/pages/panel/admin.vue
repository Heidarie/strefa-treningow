<script setup lang="ts">
const { call, session } = useApi(),
  data = ref<any>(null),
  dict = ref<any>(null),
  error = ref(""),
  message = ref(""),
  kind = ref("categories"),
  reason = ref(""),
  busy = ref(false);
const form = ref({
  slug: "",
  name: "",
  aliases: "",
  region: "",
  lat: 52,
  lng: 19,
});
useSeoMeta({
  title: "Administracja | Strefa Treningów",
  robots: "noindex,nofollow",
});
try {
  const u = await session();
  if (!u.admin) await navigateTo("/panel");
  else {
    data.value = await call("/admin");
    dict.value = await call("/dictionaries");
  }
} catch {
  await navigateTo("/konto");
}
async function run(fn: () => Promise<any>) {
  error.value = "";
  message.value = "";
  busy.value = true;
  try {
    await fn();
    data.value = await call("/admin");
    dict.value = await call("/dictionaries");
    message.value = "Zapisano.";
  } catch (e: any) {
    error.value = e.message;
  } finally {
    busy.value = false;
  }
}
async function save() {
  await run(() =>
    call("/admin/dictionaries/" + kind.value, {
      method: "PUT",
      body: {
        ...form.value,
        aliases: form.value.aliases
          .split(",")
          .map((x) => x.trim())
          .filter(Boolean),
      },
    }),
  );
}
function edit(v: any) {
  form.value = {
    slug: v.slug,
    name: v.name,
    aliases: (v.aliases || []).join(", "),
    region: v.region || "",
    lat: v.lat || 52,
    lng: v.lng || 19,
  };
}
</script>
<template>
  <div v-if="data" class="content">
    <div class="panel-header">
      <h1 style="font-size: 2.6rem">Administracja</h1>
      <NuxtLink to="/panel" class="button secondary">Panel klubu</NuxtLink>
    </div>
    <p v-if="error" class="error" role="alert">{{ error }}</p>
    <p v-if="message" class="notice">{{ message }}</p>
    <section class="form-card">
      <h2>Organizacje i publikacja</h2>
      <label
        >Uzasadnienie odrzucenia lub zawieszenia<input
          v-model="reason"
          placeholder="Informacja dla właściciela"
      /></label>
      <div v-for="o in data.organizations" :key="o.id" class="panel-row">
        <div>
          <strong>{{ o.name }}</strong
          ><small>{{ o.status }} · {{ o.reason }}</small
          ><NuxtLink
            :to="'/panel/admin-preview?organization=' + o.id"
            class="hint"
            >Podgląd danych →</NuxtLink
          >
        </div>
        <div class="inline">
          <button
            :disabled="busy"
            @click="
              run(() =>
                call('/admin/organizations/' + o.id, {
                  method: 'PUT',
                  body: { status: 'approved', reason: '' },
                }),
              )
            "
          >
            Zatwierdź</button
          ><button
            :disabled="busy || !reason"
            class="secondary"
            @click="
              run(() =>
                call('/admin/organizations/' + o.id, {
                  method: 'PUT',
                  body: { status: 'rejected', reason },
                }),
              )
            "
          >
            Odrzuć</button
          ><button
            :disabled="busy || !reason"
            class="secondary"
            @click="
              run(() =>
                call('/admin/organizations/' + o.id, {
                  method: 'PUT',
                  body: { status: 'suspended', reason },
                }),
              )
            "
          >
            Zawieś
          </button>
        </div>
      </div>
    </section>
    <section class="form-card">
      <h2>Słowniki</h2>
      <select v-model="kind" aria-label="Rodzaj słownika">
        <option value="categories">Dyscypliny</option>
        <option value="cards">Karty sportowe</option>
        <option value="cities">Miasta</option>
      </select>
      <div class="inline" style="margin: 15px 0">
        <button
          v-for="v in dict?.[kind]"
          :key="v.slug"
          class="secondary"
          @click="edit(v)"
        >
          {{ v.name }}
        </button>
      </div>
      <form class="form-grid" @submit.prevent="save">
        <label
          >Slug<input
            v-model="form.slug"
            required
            pattern="[a-z0-9-]+" /></label
        ><label>Nazwa<input v-model="form.name" required /></label
        ><label v-if="kind !== 'cards'" class="wide"
          >Aliasy, po przecinku<input v-model="form.aliases" /></label
        ><template v-if="kind === 'cities'"
          ><label>Województwo<input v-model="form.region" required /></label
          ><label
            >Szerokość<input
              v-model.number="form.lat"
              type="number"
              step="any"
              min="-90"
              max="90"
              required /></label
          ><label
            >Długość<input
              v-model.number="form.lng"
              type="number"
              step="any"
              min="-180"
              max="180"
              required /></label></template
        ><button :disabled="busy" class="wide">
          Dodaj lub zaktualizuj wpis
        </button>
      </form>
    </section>
    <section class="form-card">
      <h2>Zadania w tle</h2>
      <p v-if="!data.jobs.length" class="muted">
        Brak oczekujących zadań i błędów.
      </p>
      <div v-for="j in data.jobs" :key="j.id" class="panel-row">
        <div>
          #{{ j.id }} · {{ j.kind }} <span class="badge">{{ j.status }}</span
          ><small>Próby: {{ j.attempts }} · {{ j.error }}</small>
        </div>
        <button
          v-if="j.status === 'failed'"
          class="secondary"
          @click="
            run(() =>
              call('/admin/jobs/' + j.id + '/retry', { method: 'POST' }),
            )
          "
        >
          Ponów
        </button>
      </div>
    </section>
  </div>
</template>
