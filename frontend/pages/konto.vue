<script setup lang="ts">
// SSR stays readable; enable JS controls only after their handlers are mounted.
const ready = ref(false);
onMounted(() => {
  ready.value = true;
});
const route = useRoute(),
  { call, session } = useApi();
const mode = ref("login"),
  email = ref(""),
  password = ref(""),
  message = ref(""),
  error = ref(""),
  busy = ref(false);
try {
  await session();
} catch {}
useSeoMeta({
  title: "Konto klubu | Strefa Treningów",
  robots: "noindex,nofollow",
});
async function submit() {
  busy.value = true;
  message.value = "";
  error.value = "";
  try {
    if (mode.value === "reset") {
      const r = await call("/auth/reset-request", {
        method: "POST",
        body: { email: email.value },
      });
      message.value = r.message;
    } else if (mode.value === "register") {
      const r = await call("/auth/register", {
        method: "POST",
        body: { email: email.value, password: password.value },
      });
      message.value = r.message;
    } else {
      await call("/auth/login", {
        method: "POST",
        body: { email: email.value, password: password.value },
      });
      await session();
      if (route.query.kind === "invite") {
        message.value = "Zalogowano. Możesz teraz przyjąć zaproszenie.";
      } else await navigateTo("/panel");
    }
  } catch (e: any) {
    error.value = e.message;
  } finally {
    busy.value = false;
  }
}
async function confirm() {
  busy.value = true;
  error.value = "";
  try {
    await call("/auth/token", {
      method: "POST",
      body: { token: String(route.query.token), password: password.value },
    });
    message.value = "Gotowe. Możesz korzystać z konta.";
    await navigateTo("/konto");
    mode.value = "login";
  } catch (e: any) {
    error.value = e.message;
  } finally {
    busy.value = false;
  }
}
</script>
<template>
  <div class="content narrow">
    <span class="eyebrow">STREFA TWOJEGO KLUBU</span>
    <h1 style="font-size: 2.7rem; margin-top: 16px">
      Dobry ruch<br />dla Twojego klubu.
    </h1>
    <p class="muted" style="margin-top: 18px">
      Zarządzaj lokalizacjami i grafikiem. Daj się znaleźć osobom, które chcą
      trenować.
    </p>
    <p v-if="message" class="notice" role="status">{{ message }}</p>
    <p v-if="error" class="error" role="alert">{{ error }}</p>
    <form
      v-if="route.query.token"
      class="form-card form-stack"
      @submit.prevent="confirm"
    >
      <h2>
        {{
          route.query.kind === "invite"
            ? "Przyjmij zaproszenie"
            : route.query.kind === "reset"
              ? "Ustaw nowe hasło"
              : "Potwierdź konto"
        }}
      </h2>
      <p v-if="route.query.kind === 'invite'" class="hint">
        Najpierw zaloguj się poniżej na zaproszony e-mail. Jeśli nie masz konta,
        zarejestruj je i potwierdź adres.
      </p>
      <label v-if="route.query.kind === 'reset'"
        >Nowe hasło<input
          v-model="password"
          type="password"
          minlength="10"
          maxlength="72"
          required
          autocomplete="new-password"
          :disabled="!ready" /></label
      ><button :disabled="!ready || busy">Potwierdź</button>
    </form>
    <div class="login-modes">
      <button
        v-for="m in [
          { id: 'login', name: 'Logowanie' },
          { id: 'register', name: 'Rejestracja' },
          { id: 'reset', name: 'Reset hasła' },
        ]"
        :key="m.id"
        :class="{ secondary: mode !== m.id }"
        @click="mode = m.id"
        :disabled="!ready"
      >
        {{ m.name }}
      </button>
    </div>
    <form class="form-card form-stack" @submit.prevent="submit">
      <label
        >E-mail<input
          v-model="email"
          type="email"
          required
          autocomplete="email"
          :disabled="!ready" /></label
      ><label v-if="mode !== 'reset'"
        >Hasło<input
          v-model="password"
          type="password"
          :minlength="mode === 'register' ? 10 : undefined"
          maxlength="72"
          required
          :autocomplete="
            mode === 'register' ? 'new-password' : 'current-password'
          "
          :disabled="!ready"
      /></label>
      <p v-if="mode === 'register'" class="hint">
        Minimum 10 znaków. Na podany adres wyślemy link potwierdzający.
      </p>
      <button :disabled="!ready || busy">
        {{
          busy
            ? "Chwileczkę…"
            : mode === "login"
              ? "Zaloguj się →"
              : mode === "register"
                ? "Utwórz konto →"
                : "Wyślij link"
        }}
      </button>
    </form>
  </div>
</template>
