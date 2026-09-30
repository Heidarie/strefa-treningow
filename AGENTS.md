# Instrukcje dla agentów — Strefa Treningów

Ten plik obowiązuje w całym projekcie. Komunikuj się z użytkownikiem po polsku. Interfejs i komunikaty użytkownika są po polsku; nazwy w kodzie zachowuj zgodnie z istniejącymi konwencjami. Wprowadzaj zmiany w zakresie zadania, bez niepowiązanych refaktoryzacji.

## Kontekst i architektura

Strefa Treningów to agregator klubów i grafików dla Polski. Zapisy prowadzą na zewnętrzną stronę klubu. Płatności, rezerwacje, konta ćwiczących i importy grafików są poza obecnym MVP.

- Frontend: Vue 3, TypeScript, Nuxt 4 z SSR. Logika biznesowa i dostęp do bazy pozostają w Go.
- Backend: modularny monolit Go; jeden program z trybami `api`, `worker`, `migrate`, `seed`.
- Dane: PostgreSQL/PostGIS, zadania w trwałej kolejce PostgreSQL, media przez S3/SeaweedFS. Nie dodawaj Elasticsearch ani RabbitMQ bez potrzeby wynikającej z zadania i pomiarów.
- Mapa: MapLibre GL JS i MapTiler. Monitoring: OpenTelemetry, Prometheus, Tempo, Loki, Grafana i Alertmanager.
- Uruchomienie: Docker Compose; produkcja na jednym VPS, Caddy z TLS. Nie zakładaj wysokiej dostępności.

Najpierw sprawdź właściwy fragment [README.md](README.md), kod oraz konfigurację. [tests/VERIFICATION.md](tests/VERIFICATION.md) opisuje historyczne wyniki, a nie gwarancję aktualnego stanu środowiska.

## Mapa repozytorium

| Ścieżka | Odpowiedzialność |
| --- | --- |
| `backend/cmd/server` | Punkt wejścia i wybór trybu procesu |
| `backend/internal/app` | HTTP, auth, uprawnienia, katalog, moderacja, wyszukiwanie, grafik, media, worker i telemetria; podział według plików domenowych |
| `backend/internal/schedule` | Reguły czasu i materializacji grafiku |
| `backend/internal/media` | Walidacja i przetwarzanie obrazów |
| `backend/internal/cache` | Ograniczony cache w pamięci |
| `backend/migrations` | Schemat PostgreSQL/PostGIS |
| `frontend/pages`, `frontend/components` | Strony publiczne, konto, panel i komponenty Vue |
| `frontend/composables`, `frontend/utils` | Dostęp do API, klient typowany, paleta i funkcje pomocnicze |
| `frontend/server/routes` | Proxy API/mediów, robots i sitemap; bez reguł domenowych |
| `scripts/openapi.py` | Źródło generowanego kontraktu `openapi.json` |
| `frontend/types/api.d.ts` | Typy generowane z OpenAPI |
| `frontend/e2e`, `tests` | Playwright, akceptacja API, obciążenie i raport weryfikacji |
| `compose.yaml`, `infra`, `scripts` | Kontenery, monitoring, wdrożenie, backup i odtworzenie |

## Reguły, które należy zachować

### Uprawnienia i publikacja

- Model: organizacja → lokalizacja → oferta treningu → seria / termin. Każda operacja backendu sprawdza dostęp do właściwej organizacji; samo ukrycie przycisku nie jest kontrolą dostępu.
- Właściciel zarządza zespołem; redaktor edytuje treść placówek. Pierwsza publikacja wymaga moderacji, a zawieszenie organizacji ukrywa całą jej ofertę.
- Ukrywanie lokalizacji/treningu/serii musi działać spójnie w mapie, liście, szczegółach, mediach i sitemapie. Zmiany publikacji unieważniają cache w tej samej transakcji przez wersję `public_revision`.
- Zachowaj sesje `HttpOnly`, hashowanie haseł i tokenów, CSRF, ograniczenia żądań oraz walidację po stronie Go. Nie wprowadzaj obejść auth do testowania UI.
- Cena jest opcjonalną liczbą groszy PLN: `null` to cena niepodana, `0` to zajęcia bezpłatne. Link zapisu jest opcjonalny i dopuszcza tylko HTTP/HTTPS.

### Grafik i zadania

- Serie przechowują lokalny czas `Europe/Warsaw`; dni tygodnia używają `0=niedziela`. Terminy są materializowane na 90 dni naprzód.
- Nieistniejącą godzinę przy zmianie czasu pomijamy z ostrzeżeniem; dla godziny podwójnej wybieramy wcześniejszy moment. Nie zastępuj tej logiki prostym dodawaniem 24 godzin.
- Zmiany przyszłych wystąpień są transakcyjne i zachowują historię oraz indywidualne wyjątki. `local_date` identyfikuje pierwotne wystąpienie także po przeniesieniu. Ukryta seria ma pierwszeństwo przed wyjątkiem.
- Ponowienie zadań nie może dublować terminów ani wyników przetwarzania zdjęć. Zachowaj retry, deduplikację, odzyskiwanie blokad i widoczny stan błędu. Nie deklaruj gwarancji exactly-once dla SMTP.
- Media wymagają kontroli formatu, rozmiaru i liczby pikseli, ponownego kodowania, usunięcia metadanych i miniatur. Oryginalne uploady nie są publiczne; rekord zdjęcia i zadanie powstają atomowo.

### Frontend, mapa i SEO

- HTML SSR musi zawierać pierwsze wyniki, nagłówki i działające odnośniki bez JavaScript. Mapa jest komponentem klientowym; API nie może być zastępowane danymi demonstracyjnymi przy błędzie.
- Zachowaj canonical, metadane, dane strukturalne zgodne z treścią, sitemapę opublikowanych stron i `noindex` dla panelu oraz dodatkowych kombinacji filtrów. Awaria API nie powinna udawać 404 ani pustego katalogu.
- Mapa i lista mają wspólne filtry, ale osobne ograniczone odpowiedzi. Zachowaj paginację listy, klastry po stronie serwera, debounce i anulowanie nieaktualnych żądań mapy.
- `/konin/boks` wybiera miasto i dyscyplinę z adresu. Geolokalizacja jest na kliknięcie, nie zastępuje miasta z URL; odmowa i awaria mapy nie blokują listy. Odległość jest w linii prostej.
- MapLibre 6 wymaga importu workera przez `maplibre-gl/dist/maplibre-gl-worker.mjs?worker&url` i `setWorkerUrl` przed utworzeniem mapy. Zachowaj obsługę brakującego klucza, błędów dostawcy i sprzątanie instancji przy odmontowaniu.
- Kontrolki wymagające JavaScript włączaj dopiero po podłączeniu obsługi zdarzeń, aby hydratacja SSR nie gubiła kliknięć ani wpisanych danych. Nie blokuj przy tym odnośników i treści SSR.
- Paleta pochodzi z trwałego seeda; SSR i przeglądarka muszą używać tych samych tokenów. Zachowaj kontrast WCAG AA, etykiety, obsługę klawiaturą i responsywność.

## Zmiany kodu, API i schematu

- Formatuj zmienione pliki Go przez `gofmt`, frontend przez lokalnego Prettiera. Nie formatuj masowo niezwiązanych plików. Używaj istniejącego npm lockfile i `npm ci`.
- Wersje narzędzi sprawdzaj w `backend/go.mod`, Dockerfile, `frontend/package.json` i CI. Aktualizacje toolchainu utrzymuj spójne między tymi miejscami.
- Przy zmianie API popraw handler, `scripts/openapi.py` i klienta, a następnie z katalogu głównego wykonaj:

  ```sh
  python3 scripts/openapi.py
  cd frontend
  npm run generate:api
  ```

- Nie edytuj ręcznie `frontend/types/api.d.ts` ani samego `openapi.json` jako jedynego źródła zmiany. Sprawdź wygenerowane różnice, w tym pola opcjonalne, `null` i kody błędów.
- Obecny `App.Migrate` wykonuje pojedynczy plik wskazany przez `MIGRATION_FILE`, domyślnie `migrations/001_init.sql`. Samo dodanie `002_*.sql` nie uruchomi migracji. Zmiana schematu musi zapewniać ścieżkę aktualizacji istniejącej bazy i inicjalizacji pustej bazy; w razie potrzeby rozszerz runner oraz testy. Nie resetuj danych, aby zamaskować brak migracji.
- Zapytania SQL parametryzuj. Zmiany wyszukiwania oceniaj także pod kątem indeksów PostGIS, wielkości odpowiedzi, N+1 i unieważniania cache.

## Uruchamianie i weryfikacja

Komendy Docker wykonuj z katalogu głównego. Domyślne porty hosta: frontend `3100`, API `18080`, PostgreSQL `5432`, Mailpit `8025`, Grafana `3101`. Sprawdź zajęte porty i `.env`; nie zatrzymuj innych projektów, aby zwolnić port.

Pierwsze uruchomienie: `python3 scripts/init-env.py`, `docker compose up -d --build`, następnie `docker compose --profile tools run --rm seed`. Istniejącego `.env` nie nadpisuj. Dane demo są fikcyjne i służą wyłącznie środowisku lokalnemu.

Jeśli środowisko korzysta z monitoringu, zachowaj ten sam zestaw plików Compose przy aktualizacji API/workera:

```sh
docker compose -f compose.yaml -f infra/compose.monitoring.yaml --profile monitoring up -d --build
```

Zwykłe `docker compose up` może usunąć ustawienia dodane przez override. Przy zmianie wyłącznie frontendu można użyć `docker compose up -d --build --no-deps web`.

Dobieraj weryfikację do zmiany; dla samej dokumentacji wystarczy sprawdzenie treści i poprawności odnośników. Dla kodu:

| Zakres | Sprawdzenie |
| --- | --- |
| Backend | W `backend`: `go test ./...`, `go vet ./...`; dla integracji ustaw `TEST_DATABASE_URL` na dedykowany PostGIS i uruchom `go test -race ./...` |
| Frontend | W `frontend`: `npm run typecheck`, `npm test`, `npm run build` |
| Kontrakt API | Regeneracja OpenAPI i typów; CI sprawdza zgodność `frontend/types/api.d.ts` |
| Pełny przepływ API | Z katalogu głównego: `python3 tests/e2e.py`; wymaga lokalnego API, workera, S3, Mailpit i konta administratora z `.env` |
| UI | W `frontend`: `npx playwright test`; wymaga działającej aplikacji i ofert demonstracyjnych; w razie potrzeby zainstaluj Chromium przez `npx playwright install chromium` |
| Mapa | Osobny frontend na `3102` z testowym kluczem, następnie `MAP_TEST_URL=http://localhost:3102 npx playwright test`; dokładne komendy w README |

Bez `TEST_DATABASE_URL` test integracyjny jest pomijany, a bez `MAP_TEST_URL` pomijane są testy mapy — nie raportuj ich jako zaliczonych. Testy mapy przechwytują odpowiedzi MapTiler i nie potwierdzają działania prawdziwego klucza ani geokodowania.

Testy akceptacyjne i panelu zapisują dane: konta, organizacje, grafiki oraz media. Uruchamiaj je tylko lokalnie/testowo; nie zmieniaj ich celu na produkcję ani zewnętrzny SMTP. Benchmark i `tests/load-fixture.sql` wymagają osobnej bazy. Podawaj warunki pomiaru, w tym cache; lokalny wynik nie potwierdza celu na referencyjnym VPS.

Dodawaj test regresji dla poprawianej reguły lub błędu. Przy zmianach auth sprawdzaj izolację organizacji, przy grafiku DST i idempotencję, przy publikacji wszystkie publiczne kanały i cache, przy UI SSR i dostępność. Nie zastępuj asercji opóźnieniami ani automatycznymi retry maskującymi błąd.

## Dane, sekrety i operacje

- Nie zapisuj sekretów w kodzie, dokumentacji, logach ani odpowiedziach. `.env`, kopie zapasowe i wygenerowana konfiguracja odbiorcy alertów pozostają poza kontrolą wersji. Nie wypisuj całego `.env` ani rozwiniętego `docker compose config` z hasłami.
- Nie loguj haseł, tokenów, cookies, treści żądań ani dokładnej lokalizacji użytkownika. Zachowaj identyfikatory śladów i telemetryczne informacje o błędach bez tych danych.
- Nie wykonuj `down -v`, resetu bazy, globalnego Docker prune ani usuwania cudzych kontenerów jako rutynowego kroku. Sprzątaj tylko zasoby utworzone na potrzeby konkretnego testu, po sprawdzeniu ich projektu i nazwy.
- Backup zatrzymuje na krótko API, worker i storage. Odtworzenie przez `scripts/restore.sh` ma trafiać do nowego, odizolowanego projektu; zachowaj zabezpieczenia przed nadpisaniem działających danych.
- Nie wysyłaj rzeczywistych zapisów do klubów ani wiadomości zewnętrznych w ramach weryfikacji. Produkcyjne wdrożenie, SMTP i backup poza VPS konfiguruj wyłącznie w zakresie zleconej pracy.

## Zakończenie zadania

Podsumuj, co zmieniono, jakie sprawdzenia rzeczywiście wykonano i co pozostało niezweryfikowane. Aktualizuj README przy zmianie uruchamiania lub zachowania oraz raport weryfikacji przy nowych istotnych pomiarach. Nie przenoś historycznych wyników testów na bieżącą zmianę i nie deklaruj wdrożenia produkcyjnego na podstawie samego lokalnego Compose.
