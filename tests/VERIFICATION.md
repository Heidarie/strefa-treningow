# Weryfikacja — 2026-09-29

## Wykonano lokalnie

- `go test -race ./...` z rzeczywistym PostGIS: poprawne działanie reguł DST, przetwarzania obrazów, walidacji linków, CSRF, izolacji organizacji, unieważniania cache po ukryciu, idempotentnego generowania terminów i zachowania wyjątków.
- `go vet ./...`.
- `npm run typecheck`, `npm test`, `npm run build`.
- Audyt npm: 0 zgłoszonych podatności po aktualizacji MapLibre. `govulncheck` z Go 1.27.1: 0 podatności w wywoływanych ścieżkach; jedna podatność w zależności poza wywoływanymi ścieżkami.
- Test akceptacyjny API z Mailpit: rejestracja, potwierdzenie, logowanie, moderacja, grafik, JPEG z uploadu PNG, zaproszenie i usunięcie redaktora, reset hasła, SSR, noindex i zawieszenie.
- 18 testów Playwright (desktop/mobile): odkrywanie i szczegóły treningów, SSR bez JavaScript, odmowa geolokalizacji, brak poziomego przepełnienia, klawiatura, automatyczne WCAG AA przez axe, tworzenie organizacji/lokalizacji/treningu/serii w panelu oraz renderowanie markera i obsługa awarii dostawcy map. Po uzupełnieniu dymka o treningi i terminy ponownie zaliczono wszystkie 4 testy mapy. Kontrola wizualna zrzutów obu rozmiarów.
- Naprawiono inicjalizację mapy po SSR, bundlowanie workera MapLibre 6 i utratę kliknięć przed hydratacją. Kontrolki wymagające JavaScript są wyłączone do zamontowania obsługi zdarzeń; treść SSR i odnośniki pozostają dostępne.
- Zbudowano i uruchomiono kontenery API, worker, Nuxt, PostGIS, SeaweedFS i Mailpit.
- Uruchomiono Collector, Prometheus, Tempo, Loki, Grafanę, Alertmanager i node-exporter. Zapytanie `up` zwraca 1 dla API, Collectora i eksportera backupu; Tempo zawiera ślady aplikacji.
- Odtworzono dump oraz storage do odizolowanego projektu Docker. Liczby rekordów użytkowników, organizacji, treningów, terminów i mediów odpowiadają źródłu. SHA-256 pobranego przetworzonego zdjęcia jest identyczny.

## Obciążenie

Oddzielna baza: 10 000 lokalizacji, 10 000 treningów i 500 000 przyszłych terminów.

Końcowy pomiar: lokalny Docker na macOS, PostGIS amd64 pod emulacją, rozgrzany cache, dwa powtarzane zapytania (lista z kategorią/dniem oraz mapa z bbox).

| Miara | Wynik |
|---|---:|
| Czas | 60 s |
| Żądania | 1200 |
| Tempo | 20/s |
| p50 | 3,68 ms |
| p95 | 8,78 ms |
| Błędy | 0 |

Pierwsza próba bez cache nie spełniła celu i była dodatkowo zaburzona uruchamianiem kontenera. Po niej dodano ograniczony cache z wersjonowaniem przez transakcje PostgreSQL oraz indeks dnia tygodnia. Test integracyjny sprawdza, że zmiana publikacji unieważnia poprzednie wyniki.

## Wymaga środowiska operatora

- Rzeczywiste kafelki/geokodowanie MapTiler: brak klucza. Testy mapy używają kontrolowanych odpowiedzi dostawcy; sprawdzają lokalny worker, renderowanie punktu, popup i awarię dostawcy, ale nie zastępują weryfikacji rzeczywistego klucza.
- Pomiar p95 na referencyjnym VPS 4 vCPU / 8 GB z reprezentatywnym ruchem, w tym unikalnymi bbox i lokalizacją użytkownika. Wynik cache nie zastępuje tego pomiaru.
- TLS na docelowej domenie, dostarczalność poczty zewnętrznej i odbiorca alarmów.
- Wysłanie backupu do repozytorium poza VPS: brak danych docelowego repozytorium; zweryfikowano lokalne wykonanie i odtworzenie spójnej kopii.

## Kontrola po wznowieniu — 2026-09-30

API `/healthz`, publiczna strona `/konin/boks` oraz Grafana `/api/health` odpowiadają HTTP 200. Końcowa kontrola dostępności została wykonana po ponownym uruchomieniu API z konfiguracją monitoringu.
