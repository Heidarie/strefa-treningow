# Strefa Treningów

Agregator klubów sportowych: mapa/lista, strony SEO, oferty treningów i grafik oraz panel organizacji i moderacji. Vue 3 + Nuxt SSR, modularny backend Go, PostgreSQL/PostGIS. Bez Elasticsearch i RabbitMQ.

## Uruchomienie lokalne

Wymagania: Docker Engine/Desktop i Docker Compose **2.24.4+**. Porty domyślne dobrano tak, aby nie kolidowały z aplikacjami działającymi na 3000 i 8080.

```sh
python3 scripts/init-env.py
docker compose up -d --build
docker compose --profile tools run --rm seed
```

- Aplikacja: http://localhost:3100
- Panel: http://localhost:3100/panel
- API: http://localhost:18080/api/v1
- Testowa poczta Mailpit: http://localhost:8025
- Logowanie administratora: `ADMIN_EMAIL` i `ADMIN_PASSWORD` z lokalnego `.env`.

`seed` tworzy administratora. Przy `SEED_DEMO=true` tworzy też wyraźnie oznaczoną organizację demonstracyjną z trzema przykładowymi ofertami w Poznaniu i Koninie. To fikcyjne dane, nie rzeczywiste kluby. Nie uruchamiaj danych demonstracyjnych na produkcji. Ponowne wykonanie seeda nie nadpisuje hasła istniejącego użytkownika.

`init-env.py` nie nadpisuje istniejącego pliku. Generuje trwały, losowy seed palety. Paleta jest jednakowa w SSR i przeglądarce; test kontrastu sprawdza wszystkie możliwe odcienie akcentu.

### Mapa

MapLibre 6 ładuje worker przez bundler Vite (`?worker&url`), zgodnie z [instrukcją dostawcy](https://maplibre.org/maplibre-gl-js/docs/). Worker jest częścią lokalnej kompilacji.

W `.env` ustaw `MAPTILER_KEY`, następnie:

```sh
docker compose up -d api web worker
```

Opcjonalnie ustaw osobny `NUXT_PUBLIC_MAPTILER_KEY` ograniczony do domeny strony; w przeciwnym razie frontend używa `MAPTILER_KEY`. Klucz przeglądarkowy jest publiczny z założenia. Klucz backendu służy do geokodowania adresów. Atrybucja dostawcy pozostaje widoczna.

Bez klucza strona działa z listą i komunikatem w obszarze mapy. Lokalizacje można wtedy wprowadzać ręcznie przez współrzędne. Nie udajemy działającej mapy ani wyników geokodowania.

### Pierwszy klub

1. Otwórz `/konto`, zarejestruj konto i potwierdź adres linkiem z Mailpit.
2. W panelu utwórz organizację, lokalizację i trening.
3. Dodaj serię cotygodniową albo wydarzenie jednorazowe.
4. Zgłoś organizację do publikacji.
5. Administrator zatwierdza ją w `/panel/admin`; dostępny jest podgląd treści przed zatwierdzeniem.
6. Organizacja pojawi się na mapie/liście i stronach miasta/dyscypliny.

Właściciel zarządza organizacją i zaprasza redaktorów. Redaktor może edytować wszystkie jej lokalizacje, ale nie zarządza zespołem. Zaproszona osoba musi mieć potwierdzone konto na zaproszony e-mail i być zalogowana przy akceptacji zaproszenia.

Początkowy słownik zawiera osiem miast. Administrator może dodawać następne, wraz z województwem, współrzędnymi i aliasami. Miejscowości o tej samej nazwie dostają różne slugi; wyszukiwarka wymaga wtedy jednoznacznego wyboru.

## Architektura

```text
Przeglądarka → Caddy → Nuxt SSR (Vue)
                    → Go REST API → PostgreSQL + PostGIS
                                  → S3 / SeaweedFS
                         Go worker → PostgreSQL jobs / SMTP / obrazy / grafik
API i worker → OpenTelemetry Collector → Tempo / Loki
API metrics  → Prometheus → Grafana / Alertmanager
```

Kod backendu jest jednym modułem Go i jednym obrazem z poleceniami `api`, `worker`, `migrate`, `seed`. Warstwy HTTP i koordynacja domen znajdują się w `backend/internal/app`, rozdzielone według odpowiedzialności (`auth`, `catalog`, `schedules`, `search`, `media`, `worker`). Czyste reguły czasu, przetwarzanie obrazów i cache są w osobnych pakietach `internal/schedule`, `internal/media`, `internal/cache`. Wszystkie reguły uprawnień i zapisów są w Go, nie w Nuxt.

- Relacje: organizacja → lokalizacja → trening → seria / termin.
- Cena: opcjonalna liczba groszy PLN; interfejs edytuje złote. `null` oznacza niepodaną cenę, `0` zajęcia bezpłatne.
- Kategorie i karty to słowniki administratora. Logo lokalizacji może dziedziczyć logo firmy.
- Każda operacja edycji sprawdza członkostwo po stronie backendu. Sesje i jednorazowe tokeny mają hashowane identyfikatory w bazie; hasła używają bcrypt. Mutacje panelu wymagają CSRF.
- Weryfikacja i reset hasła oraz zaproszenia wygasają po 24 godzinach. Sesja wygasa po 7 dniach; reset hasła unieważnia sesje.
- Adres zapisu dopuszcza wyłącznie HTTP/HTTPS. Aplikacja nie wysyła zgłoszenia do klubu — pokazuje link.
- Ukrywanie lub zawieszenie usuwa treść z publicznych odpowiedzi i sitemapy. Nieopublikowane zdjęcia są dostępne wyłącznie uprawnionym członkom i administratorowi.

### Grafik

Serie zawierają dni tygodnia (`0=niedziela`), lokalną godzinę w `Europe/Warsaw`, czas trwania i zakres dat. Daty materializowane są na 90 dni do przodu. Codzienne zadanie uzupełnia horyzont.

Zmiana serii przebudowuje przyszłe zwykłe wystąpienia w transakcji, zachowując historię i indywidualne wyjątki. Klucz `(series_id, local_date)` zapobiega duplikatom. `local_date` identyfikuje pierwotne wystąpienie także po przeniesieniu go na inny dzień. Ukryta seria ma pierwszeństwo przed widocznością wyjątku.

Nieistniejąca godzina podczas przejścia na czas letni jest pomijana i sygnalizowana w panelu. Dla podwójnej godziny jesienią wybierany jest wcześniejszy moment. Wszystkie publiczne godziny prezentowane są w czasie Polski.

### Wyszukiwanie i SEO

- `/`, `/{miasto}`, `/{miasto}/{dyscyplina}`, `/lokalizacja/{id}`, `/trening/{id}`.
- Pierwsza strona wyników i linki znajdują się w HTML SSR; mapa jest ładowana po stronie klienta.
- Mapa i lista korzystają z tych samych warunków publikacji, dyscypliny, karty, dnia i obszaru. Mapa zwraca klastry z obszaru widoku, nigdy cały katalog.
- Lista: 20 lokalizacji na stronę. Bogate dane treningów są pobierane dopiero dla wybranej strony.
- GiST na współrzędnych, indeksy lokalizacji, kategorii i kart oraz terminów/dnia tygodnia.
- Cache publicznych odczytów: maksymalnie 32 MiB / 512 wpisów / 2 minuty. Każde żądanie sprawdza wersję publikacji w PostgreSQL. Triggery zmieniają ją w tej samej transakcji co treść, więc cache nie czeka na TTL po ukryciu oferty. Zapytania zawierające lokalizację użytkownika nie są cache’owane.
- Geolokalizacja jest wyłącznie na żądanie. Nie zastępuje miasta z URL; dystans jest w linii prostej. Współrzędne użytkownika nie trafiają do logów.
- Dodatkowe parametry filtrów i puste strony miast mają `noindex`; canonical prowadzi do strony bez parametrów. Sitemap zawiera wyłącznie opublikowaną ofertę. Awaria backendu zwraca 503 zamiast pozornego 404.

### Zadania i zdjęcia

Kolejka PostgreSQL korzysta z `FOR UPDATE SKIP LOCKED`, deduplikacji zadań, maksymalnie 5 prób i odzyskiwania wygasłych blokad. Administrator widzi błędy i może ponowić zadanie. Przetwarzanie obrazów i uzupełnianie terminów są idempotentne. SMTP działa co najmniej raz: awaria po przyjęciu wiadomości przez serwer pocztowy, ale przed zapisem sukcesu, może spowodować powtórne dostarczenie tego samego linku.

Upload: JPEG/PNG do 8 MiB i 25 megapikseli. Worker ponownie koduje pliki do JPEG, usuwa metadane i tworzy warianty do 1920 oraz 480 px. Oryginały nie są publiczne. Utworzenie rekordu zdjęcia i zadania jest atomowe; media są przechowywane w S3, nie w PostgreSQL.

## API i praca nad kodem

Kontrakt: `openapi.json`. Generowany klient typowany: `frontend/types/api.d.ts`, adapter: `frontend/utils/client.ts`.

```sh
python3 scripts/openapi.py
cd frontend
npm ci
npm run generate:api
npm run typecheck
npm test
npm run build
```

```sh
cd backend
# Go 1.27.1 (wersja także w obrazie Docker i CI)
go test ./...
go vet ./...
# Integracja z lokalną bazą (użyj hasła ze swojego .env):
TEST_DATABASE_URL='postgres://strefa:strefa-local-password@localhost:5432/strefa?sslmode=disable' go test -race ./...
```

Testy integracyjne tworzą własne dane i usuwają je po zakończeniu. Bez `TEST_DATABASE_URL` ten test jest jawnie pomijany. CI uruchamia go z osobnym PostGIS.

```sh
# Pełny przepływ API + Mailpit + przetwarzanie zdjęcia:
python3 tests/e2e.py
# Interfejs desktop/mobile (wymaga demonstracyjnych ofert):
cd frontend
npx playwright install chromium
npx playwright test
```

Testy mapy korzystają z kontrolowanego podkładu i odpowiedzi 503; nie potrzebują rzeczywistego klucza ani żądań do MapTiler. Po `npm run build` uruchom w osobnym terminalu:

```sh
PORT=3102 NUXT_API_BASE=http://localhost:18080 \
NUXT_PUBLIC_MAPTILER_KEY=test-only NUXT_PUBLIC_SITE_URL=http://localhost:3102 \
node .output/server/index.mjs
```

Następnie `MAP_TEST_URL=http://localhost:3102 npx playwright test` uruchamia także testy markerów i awarii mapy na desktop/mobile. Bez `MAP_TEST_URL` te cztery przypadki są jawnie pomijane.

Test API tworzy unikalne konta i organizację testową; na końcu ją zawiesza. Nie wysyła wiadomości poza lokalny Mailpit ani żądań zapisu do zewnętrznego klubu.

## Monitoring

```sh
docker compose -f compose.yaml -f infra/compose.monitoring.yaml --profile monitoring up -d
```

Grafana: http://localhost:3101 (`admin`, hasło `GRAFANA_ADMIN_PASSWORD`). Dostarczone są źródła danych, dashboard i reguły alertów: niedostępność API, błędy HTTP, opóźniona kolejka, nieudane zadania i brak backupu.

Ślady obejmują HTTP, zapytania PostgreSQL i zadania. Metryki API są na wewnętrznym `/metrics`. Logi JSON zawierają identyfikatory śladów, bez treści żądań i parametrów SQL. Collector czyta tylko współdzielony wolumen logów Strefy, nie logi innych kontenerów. Pliki aplikacji rotują po 10 MiB, maksymalnie trzy archiwa na proces.

Lokalny Alertmanager pokazuje alerty bez wysyłania e-maili. Dla produkcji ustaw `ALERT_EMAIL` i SMTP, wykonaj `python3 scripts/configure-alerts.py`. Powstaje ignorowany przez Git plik z konfiguracją odbiorcy. Skrypt sam nie wysyła wiadomości.

## Wdrożenie na VPS

1. Skopiuj projekt do `/opt/strefa-treningow`, zainstaluj Docker/Compose i skonfiguruj DNS domeny.
2. Wygeneruj `.env`. Ustaw `PUBLIC_URL=https://twoja-domena.pl`, `DOMAIN`, silne hasła bazy/S3/admina/Grafany, klucze map, SMTP z TLS, `ALERT_EMAIL`, `SEED_DEMO=false` oraz repozytorium i hasło Restic. `DATABASE_URL` musi używać tego samego hasła co `POSTGRES_PASSWORD` (znaki specjalne zakodowane w URL).
3. Uruchom walidację i konfigurację alertów:

```sh
python3 scripts/check-production.py
python3 scripts/configure-alerts.py
docker compose -f compose.yaml -f infra/compose.production.yaml --profile production --profile monitoring up -d --build
docker compose --profile tools run --rm seed
```

4. Publicznie udostępnij tylko 80/443. Baza, API i Grafana mają lokalne bindy; administruj nimi przez tunel SSH. Caddy automatycznie obsługuje TLS.
5. Ustaw backup według instrukcji poniżej i sprawdź odbiorcę alertów. Zewnętrzny SMTP, klucz MapTiler, domena, VPS i repozytorium backupu wymagają rzeczywistych danych operatora — repozytorium nie zawiera ich zamienników.

Na Apple Silicon PostGIS używa obrazu amd64 pod emulacją. Na VPS x86_64 działa natywnie. Pełny zestaw monitoringu zwiększa zużycie pamięci; dobierz limit kontenerów po pomiarze na docelowym serwerze. Jeden VPS nie zapewnia wysokiej dostępności.

### Backup i odtworzenie

Wymagany jest Restic na hoście. `RESTIC_REPOSITORY` musi wskazywać lokalizację poza VPS, a `RESTIC_PASSWORD` należy przechować także poza serwerem. W przypadku S3 ustaw odpowiednie `AWS_ACCESS_KEY_ID` i `AWS_SECRET_ACCESS_KEY` dla repozytorium backupu.

```sh
# Po bezpiecznym wczytaniu zmiennych Restic z .env, jednorazowo:
restic init
# Wykonanie kopii:
./scripts/backup.sh
```

Skrypt na krótko zatrzymuje API, worker i storage, tworzy spójny dump oraz archiwum danych S3, wznawia aplikację, wysyła kopię przez Restic, stosuje retencję 14 dni i sprawdza repozytorium. Wymaga shell-compatible `.env`. Plik metryki sukcesu aktualizuje się dopiero po całym poprawnym backupie. To celowa, krótka przerwa w dostępności zapisów i zdjęć w wariancie jednego VPS.

Zainstaluj `infra/backup.service` i `infra/backup.timer` w `/etc/systemd/system/`, następnie `systemctl enable --now backup.timer`. Harmonogram to 03:15 w strefie czasu hosta. Ustaw na VPS `Europe/Warsaw`, jeśli kopia ma powstawać według czasu Polski.

Odtwórz pliki z Restic do osobnego katalogu, następnie:

```sh
RESTORE_PROJECT=strefa-restore-20260929 \
RESTORE_DATABASE=/absolute/path/database.dump \
RESTORE_STORAGE=/absolute/path/storage.tar.gz \
./scripts/restore.sh
```

Skrypt odmawia użycia istniejącego projektu i nazwy głównej aplikacji. Tworzy odizolowane wolumeny bez portów hosta i odtwarza bazę z `template0`, aby uniknąć konfliktów schematów PostGIS. Po odtworzeniu porównaj liczby rekordów i sumy kontrolne obrazów; dopiero potem przełącz ruch. Nie nadpisuje działającej bazy.

## Weryfikacja i wydajność

Raport wykonanych sprawdzeń: `tests/VERIFICATION.md`. Scenariusze obciążenia: `tests/load.js` (k6) oraz `tests/benchmark.py` (standardowy Python). Dane: `tests/load-fixture.sql` — uruchamiaj wyłącznie na oddzielnej bazie testowej.

Cel dla docelowego VPS: p95 <300 ms przy 20 żądaniach/s, 10 tys. lokalizacji i 500 tys. terminów. Lokalny pomiar powtarzanych zapytań po rozgrzaniu cache osiągnął p95 8,78 ms, 1200 żądań i 0 błędów. To pomiar konkretnego scenariusza, nie gwarancja dla nowych kombinacji filtrów, geolokalizacji, wszystkich obszarów mapy ani wydajności docelowego VPS. Przed uruchomieniem produkcji powtórz testy z reprezentatywnym ruchem na serwerze docelowym.
