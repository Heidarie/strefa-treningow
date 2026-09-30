<script setup lang="ts">
import { dateLabel, warsawDate } from "~/utils/theme";
const { call, session } = useApi(),
  me = ref<any>(null),
  data = ref<any>(null),
  dict = ref<any>(null),
  tab = ref("locations"),
  orgID = ref(""),
  error = ref(""),
  message = ref(""),
  busy = ref(false);
const form = ref<any>(null),
  formKind = ref(""),
  newOrg = ref(""),
  inviteEmail = ref(""),
  geoResults = ref<any[]>([]);
useSeoMeta({
  title: "Panel klubu | Strefa Treningów",
  robots: "noindex,nofollow",
});
try {
  me.value = await session();
  [data.value, dict.value] = await Promise.all([
    call("/panel"),
    call("/dictionaries"),
  ]);
  orgID.value = data.value.organizations[0]?.id || "";
} catch {
  await navigateTo("/konto");
}
const org = computed(() =>
  data.value?.organizations.find((o: any) => o.id === orgID.value),
);
const locations = computed(
  () =>
    data.value?.locations.filter(
      (l: any) => l.organization_id === orgID.value,
    ) || [],
);
const trainings = computed(
  () =>
    data.value?.trainings.filter((t: any) =>
      locations.value.some((l: any) => l.id === t.location_id),
    ) || [],
);
const series = computed(
  () =>
    data.value?.series.filter((s: any) =>
      trainings.value.some((t: any) => t.id === s.training_id),
    ) || [],
);
const occurrences = computed(
  () =>
    data.value?.occurrences.filter((s: any) =>
      trainings.value.some((t: any) => t.id === s.training_id),
    ) || [],
);
const members = computed(
  () =>
    data.value?.members.filter((m: any) => m.organization_id === orgID.value) ||
    [],
);
const days = ["Nd", "Pn", "Wt", "Śr", "Cz", "Pt", "So"];
const statusNames: Record<string, string> = {
  draft: "Szkic",
  pending: "Czeka na zatwierdzenie",
  approved: "Opublikowana",
  rejected: "Odrzucona",
  suspended: "Zawieszona",
};
async function refresh() {
  data.value = await call("/panel");
}
async function action(fn: () => Promise<any>, text = "Zapisano zmiany.") {
  busy.value = true;
  error.value = "";
  message.value = "";
  try {
    await fn();
    await refresh();
    message.value = text;
  } catch (e: any) {
    error.value = e.message;
  } finally {
    busy.value = false;
  }
}
async function addOrg() {
  await action(async () => {
    const r = await call("/organizations", {
      method: "POST",
      body: { name: newOrg.value },
    });
    orgID.value = r.id;
    newOrg.value = "";
  }, "Organizacja utworzona. Dodaj lokalizację i treningi.");
}
function edit(kind: string, item?: any) {
  formKind.value = kind;
  geoResults.value = [];
  const defaults: any = {
    locations: {
      organization_id: orgID.value,
      name: "",
      city: "poznan",
      address: "",
      logo: "",
      lat: 52.4064,
      lng: 16.9252,
      hidden: false,
    },
    trainings: {
      location_id: locations.value[0]?.id || "",
      name: "",
      category: "pilates",
      description: "",
      price: null,
      cards: [],
      photos: [],
      signup_url: "",
      hidden: false,
    },
    schedules: {
      training_id: trainings.value[0]?.id || "",
      start_date: warsawDate(),
      end_date: "",
      weekdays: [1, 3],
      local_time: "18:00",
      duration: 60,
      hidden: false,
      one_off: false,
    },
    organizations: { name: org.value?.name || "", logo: org.value?.logo || "" },
  };
  form.value = JSON.parse(JSON.stringify(item || defaults[kind]));
  if (kind === "schedules") {
    form.value.start_date = form.value.start_date?.slice(0, 10);
    form.value.end_date = form.value.end_date?.slice(0, 10) || "";
    form.value.one_off = false;
  }
  if (kind === "trainings")
    form.value.pricePLN =
      form.value.price === null ? "" : form.value.price / 100;
  if (kind === "organizations") form.value.id = orgID.value;
}
function editOccurrence(o: any) {
  const fmt = new Intl.DateTimeFormat("sv-SE", {
    timeZone: "Europe/Warsaw",
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    hourCycle: "h23",
  }).format(new Date(o.starts_at));
  formKind.value = "occurrences";
  form.value = {
    id: o.id,
    date: fmt.slice(0, 10),
    local_time: fmt.slice(11, 16),
    duration: Math.round(
      (+new Date(o.ends_at) - +new Date(o.starts_at)) / 60000,
    ),
    hidden: o.hidden,
  };
}
async function save() {
  const fields: Record<string, string[]> = {
    organizations: ["name", "logo"],
    locations: [
      "organization_id",
      "name",
      "city",
      "address",
      "logo",
      "lat",
      "lng",
      "hidden",
    ],
    trainings: [
      "location_id",
      "name",
      "category",
      "description",
      "price",
      "cards",
      "photos",
      "signup_url",
      "hidden",
    ],
    schedules: [
      "training_id",
      "start_date",
      "end_date",
      "weekdays",
      "local_time",
      "duration",
      "hidden",
      "one_off",
    ],
    occurrences: ["date", "local_time", "duration", "hidden"],
  };
  const b = Object.fromEntries(
    fields[formKind.value]!.map((k) => [k, form.value[k]]),
  );
  if (formKind.value === "trainings")
    b.price =
      form.value.pricePLN === ""
        ? null
        : Math.round(Number(form.value.pricePLN) * 100);
  const id = form.value.id;
  await action(async () => {
    await call("/" + formKind.value + (id ? "/" + id : ""), {
      method: id ? "PUT" : "POST",
      body: b,
    });
    form.value = null;
  });
}
async function geocode() {
  error.value = "";
  try {
    geoResults.value = (
      await call("/geocode", {
        query: {
          q:
            form.value.address +
            " " +
            (dict.value.cities.find((c: any) => c.slug === form.value.city)
              ?.name || ""),
        },
      })
    ).features;
  } catch (e: any) {
    error.value = e.message;
  }
}
function chooseGeo(g: any) {
  form.value.lng = g.center[0];
  form.value.lat = g.center[1];
  geoResults.value = [];
}
let interval: ReturnType<typeof setInterval> | undefined;
onMounted(() => {
  interval = setInterval(() => {
    if (data.value?.media.some((m: any) => m.status === "pending"))
      refresh().catch(() => {});
  }, 4000);
});
onBeforeUnmount(() => clearInterval(interval));
watch(orgID, () => {
  form.value = null;
});
</script>
<template>
  <div v-if="data" class="content">
    <div class="panel-header">
      <div>
        <span class="eyebrow">TWÓJ KLUB, TWÓJ GRAFIK</span>
        <h1 style="margin-top: 10px">Panel organizacji</h1>
      </div>
      <div class="inline">
        <NuxtLink v-if="me?.admin" to="/panel/admin" class="button secondary"
          >Administracja</NuxtLink
        ><button
          class="ghost"
          @click="
            action(async () => {
              await call('/auth/logout', { method: 'POST' });
              await navigateTo('/konto');
            })
          "
        >
          Wyloguj
        </button>
      </div>
    </div>
    <p v-if="error" class="error" role="alert">{{ error }}</p>
    <p v-if="message" class="notice" role="status">{{ message }}</p>
    <div class="inline" style="margin-bottom: 20px">
      <select v-model="orgID" aria-label="Organizacja" style="max-width: 350px">
        <option v-for="o in data.organizations" :key="o.id" :value="o.id">
          {{ o.name }}
        </option></select
      ><span v-if="org" class="badge">{{ statusNames[org.status] }}</span
      ><button
        v-if="
          org?.role === 'owner' && ['draft', 'rejected'].includes(org.status)
        "
        :disabled="busy"
        @click="
          action(
            () =>
              call('/organizations/' + orgID + '/submit', { method: 'POST' }),
            'Wysłano do zatwierdzenia.',
          )
        "
      >
        Zgłoś do publikacji
      </button>
    </div>
    <p v-if="org?.reason" class="notice">
      Informacja administratora: {{ org.reason }}
    </p>
    <details class="form-card" :open="!data.organizations.length">
      <summary>Utwórz organizację</summary>
      <form class="inline" style="margin-top: 15px" @submit.prevent="addOrg">
        <input
          v-model="newOrg"
          placeholder="Nazwa firmy lub klubu"
          aria-label="Nazwa organizacji"
          required
          style="max-width: 400px"
        /><button :disabled="busy">Utwórz</button>
      </form>
    </details>
    <template v-if="org"
      ><div class="tabs" style="margin-top: 25px">
        <button
          v-for="t in [
            { id: 'locations', name: 'Lokalizacje' },
            { id: 'trainings', name: 'Treningi' },
            { id: 'schedules', name: 'Grafik' },
            { id: 'team', name: 'Zespół i firma' },
          ]"
          :key="t.id"
          :class="{ active: tab === t.id }"
          @click="
            tab = t.id;
            form = null;
          "
        >
          {{ t.name }}
        </button>
      </div>
      <template v-if="!form"
        ><section v-if="tab === 'locations'" class="form-card">
          <div class="panel-header">
            <h2>Twoje lokalizacje</h2>
            <button @click="edit('locations')">+ Dodaj lokalizację</button>
          </div>
          <div v-for="l in locations" :key="l.id" class="panel-row">
            <div>
              <strong>{{ l.name }}</strong
              ><small
                >{{ l.address }} ·
                {{
                  l.hidden ? "Ukryta" : "Widoczna po zatwierdzeniu firmy"
                }}</small
              >
            </div>
            <button class="secondary" @click="edit('locations', l)">
              Edytuj
            </button>
          </div>
          <p v-if="!locations.length" class="muted">
            Dodaj pierwszą lokalizację, aby utworzyć ofertę treningów.
          </p>
        </section>
        <section v-if="tab === 'trainings'" class="form-card">
          <div class="panel-header">
            <h2>Oferta treningów</h2>
            <button :disabled="!locations.length" @click="edit('trainings')">
              + Dodaj trening
            </button>
          </div>
          <div v-for="t in trainings" :key="t.id" class="panel-row">
            <div>
              <strong>{{ t.name }}</strong
              ><small
                >{{
                  locations.find((l: any) => l.id === t.location_id)?.name
                }}
                · {{ t.hidden ? "Ukryty" : "Widoczny" }}</small
              >
            </div>
            <button class="secondary" @click="edit('trainings', t)">
              Edytuj
            </button>
          </div>
          <p v-if="!trainings.length" class="muted">
            Dodaj zajęcia i opisz, czego uczestnicy mogą się spodziewać.
          </p>
        </section>
        <section v-if="tab === 'schedules'" class="form-card">
          <div class="panel-header">
            <h2>Grafik zajęć</h2>
            <button :disabled="!trainings.length" @click="edit('schedules')">
              + Dodaj do grafiku
            </button>
          </div>
          <div
            v-for="w in data.warnings.filter((w: any) =>
              series.some((s: any) => s.id === w.series_id),
            )"
            :key="w.series_id + w.local_date"
            class="notice"
          >
            {{ w.local_date }}: {{ w.message }}
          </div>
          <h3>Serie tygodniowe</h3>
          <div v-for="s in series" :key="s.id" class="panel-row">
            <div>
              <strong>{{
                trainings.find((t: any) => t.id === s.training_id)?.name
              }}</strong
              ><small
                >{{ s.weekdays.map((d: number) => days[d]).join(", ") }} ·
                {{ s.local_time }} · {{ s.duration }} min
                {{ s.hidden ? "· Ukryta" : "" }}</small
              >
            </div>
            <button class="secondary" @click="edit('schedules', s)">
              Edytuj przyszłe terminy
            </button>
          </div>
          <h3 style="margin-top: 25px">Pojedyncze terminy</h3>
          <div v-for="o in occurrences" :key="o.id" class="panel-row">
            <div>
              <strong>{{
                trainings.find((t: any) => t.id === o.training_id)?.name
              }}</strong
              ><small
                >{{ dateLabel(o.starts_at) }} {{ o.hidden ? "· Ukryty" : "" }}
                {{ o.overridden ? "· Wyjątek" : "" }}</small
              >
            </div>
            <button class="secondary" @click="editOccurrence(o)">
              Zmień termin
            </button>
          </div>
        </section>
        <section v-if="tab === 'team'" class="form-card">
          <div class="panel-header">
            <h2>{{ org.name }}</h2>
            <button v-if="org.role === 'owner'" @click="edit('organizations')">
              Edytuj firmę i logo
            </button>
          </div>
          <div v-for="m in members" :key="m.user_id" class="panel-row">
            <div>
              {{ m.email }}
              <span class="badge">{{
                m.role === "owner" ? "Właściciel" : "Redaktor"
              }}</span>
            </div>
            <button
              v-if="org.role === 'owner' && m.role === 'editor'"
              class="secondary"
              @click="
                action(() =>
                  call('/organizations/' + orgID + '/members/' + m.user_id, {
                    method: 'DELETE',
                  }),
                )
              "
            >
              Usuń dostęp
            </button>
          </div>
          <form
            v-if="org.role === 'owner'"
            class="form-stack"
            style="margin-top: 24px"
            @submit.prevent="
              action(
                () =>
                  call('/organizations/' + orgID + '/invite', {
                    method: 'POST',
                    body: { email: inviteEmail },
                  }),
                'Wysłano zaproszenie.',
              )
            "
          >
            <label
              >E-mail nowego redaktora<input
                v-model="inviteEmail"
                type="email"
                required /></label
            ><button :disabled="busy">Zaproś redaktora</button>
          </form>
        </section></template
      >
      <form v-else class="form-card form-stack" @submit.prevent="save">
        <div class="panel-header">
          <h2>
            {{ form.id ? "Edytuj" : "Dodaj" }}
            {{
              {
                locations: "lokalizację",
                trainings: "trening",
                schedules: "grafik",
                organizations: "organizację",
                occurrences: "termin",
              }[formKind]
            }}
          </h2>
          <button type="button" class="ghost" @click="form = null">
            Anuluj
          </button>
        </div>
        <div
          v-if="['locations', 'trainings', 'organizations'].includes(formKind)"
          class="form-grid"
        >
          <label class="wide"
            >Nazwa<input v-model="form.name" required maxlength="200"
          /></label>
          <template v-if="formKind === 'locations'"
            ><label
              >Miasto<select v-model="form.city">
                <option v-for="c in dict.cities" :key="c.slug" :value="c.slug">
                  {{ c.name }} ({{ c.region }})
                </option>
              </select></label
            ><label>Adres<input v-model="form.address" required /></label
            ><button type="button" class="secondary wide" @click="geocode">
              Znajdź adres na mapie
            </button>
            <div v-if="geoResults.length" class="wide">
              <button
                v-for="g in geoResults"
                :key="g.id"
                type="button"
                class="secondary"
                @click="chooseGeo(g)"
              >
                {{ g.place_name }}
              </button>
            </div>
            <label
              >Szerokość geograficzna<input
                v-model.number="form.lat"
                type="number"
                step="any"
                min="-90"
                max="90"
                required /></label
            ><label
              >Długość geograficzna<input
                v-model.number="form.lng"
                type="number"
                step="any"
                min="-180"
                max="180"
                required
            /></label>
            <div class="wide">
              <ClientOnly
                ><LocationPicker
                  :lat="form.lat"
                  :lng="form.lng"
                  @change="
                    (v) => {
                      form.lat = v.lat;
                      form.lng = v.lng;
                    }
                  "
              /></ClientOnly>
              <p class="hint">
                Potwierdź pinezkę kliknięciem lub przesunięciem.
              </p>
            </div></template
          >
          <template v-if="formKind === 'trainings'"
            ><label
              >Lokalizacja<select v-model="form.location_id" required>
                <option v-for="l in locations" :key="l.id" :value="l.id">
                  {{ l.name }}
                </option>
              </select></label
            ><label
              >Kategoria<select v-model="form.category">
                <option
                  v-for="c in dict.categories"
                  :key="c.slug"
                  :value="c.slug"
                >
                  {{ c.name }}
                </option>
              </select></label
            ><label class="wide"
              >Opis<textarea
                v-model="form.description"
                maxlength="10000"
              /></label
            ><label
              >Cena w złotych (opcjonalnie)<input
                v-model.number="form.pricePLN"
                type="number"
                min="0"
                step="0.01"
                placeholder="45,00" /></label
            ><label
              >Link zapisu (opcjonalnie)<input
                v-model="form.signup_url"
                type="url"
                placeholder="https://"
            /></label>
            <fieldset class="wide">
              <legend>Obsługiwane karty</legend>
              <div class="inline">
                <label v-for="c in dict.cards" :key="c.slug"
                  ><input
                    v-model="form.cards"
                    type="checkbox"
                    :value="c.slug"
                  />{{ c.name }}</label
                >
              </div>
            </fieldset></template
          >
          <div v-if="formKind !== 'trainings'" class="wide">
            <label
              >Logo<select v-model="form.logo">
                <option value="">
                  {{
                    formKind === "locations"
                      ? "Dziedzicz logo organizacji"
                      : "Bez logo"
                  }}
                </option>
                <option
                  v-for="m in data.media.filter(
                    (m: any) =>
                      m.organization_id === orgID && m.status === 'ready',
                  )"
                  :key="m.id"
                  :value="m.url"
                >
                  Zdjęcie {{ m.id.slice(0, 8) }}
                </option>
              </select></label
            ><img
              v-if="form.logo"
              :src="form.logo"
              alt="Wybrane logo"
              width="70"
              style="margin-top: 10px"
            />
          </div>
          <div class="wide">
            <MediaLibrary
              :organization-id="orgID"
              :items="data.media"
              :model-value="
                formKind === 'trainings'
                  ? form.photos
                  : form.logo
                    ? [form.logo]
                    : []
              "
              @update:model-value="
                (v) => {
                  if (formKind === 'trainings') form.photos = v;
                  else form.logo = v.at(-1) || '';
                }
              "
              @refresh="refresh"
            />
          </div>
        </div>
        <template v-if="formKind === 'schedules'"
          ><label
            >Trening<select v-model="form.training_id" :disabled="!!form.id">
              <option v-for="t in trainings" :key="t.id" :value="t.id">
                {{ t.name }}
              </option>
            </select></label
          ><label v-if="!form.id" class="inline"
            ><input v-model="form.one_off" type="checkbox" />Wydarzenie
            jednorazowe</label
          >
          <div class="form-grid">
            <label
              >{{ form.one_off ? "Data wydarzenia" : "Początek serii"
              }}<input v-model="form.start_date" type="date" required /></label
            ><label v-if="!form.one_off"
              >Koniec serii (opcjonalnie)<input
                v-model="form.end_date"
                type="date"
            /></label>
          </div>
          <fieldset v-if="!form.one_off">
            <legend>Dni tygodnia</legend>
            <div class="inline">
              <label v-for="(d, i) in days" :key="i"
                ><input v-model="form.weekdays" type="checkbox" :value="i" />{{
                  d
                }}</label
              >
            </div>
          </fieldset>
          <p v-if="form.id" class="hint">
            Zmiana dotyczy przyszłych wystąpień. Indywidualne wyjątki i historia
            zostają zachowane.
          </p></template
        >
        <label v-if="formKind === 'occurrences'"
          >Data<input v-model="form.date" type="date" required
        /></label>
        <div
          v-if="['schedules', 'occurrences'].includes(formKind)"
          class="form-grid"
        >
          <label
            >Godzina (Europe/Warsaw)<input
              v-model="form.local_time"
              type="time"
              required /></label
          ><label
            >Czas trwania w minutach<input
              v-model.number="form.duration"
              type="number"
              min="5"
              max="1440"
              required
          /></label>
        </div>
        <label v-if="formKind !== 'organizations'" class="inline"
          ><input v-model="form.hidden" type="checkbox" />Ukryj
          {{
            formKind === "schedules"
              ? "serię i jej przyszłe terminy"
              : "przed użytkownikami"
          }}</label
        ><button :disabled="busy">
          {{ busy ? "Zapisywanie…" : "Zapisz zmiany" }}
        </button>
      </form></template
    >
  </div>
</template>
