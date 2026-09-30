CREATE EXTENSION IF NOT EXISTS postgis;
CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS unaccent;
CREATE TABLE IF NOT EXISTS users (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), email text UNIQUE NOT NULL, password text NOT NULL,
 verified boolean NOT NULL DEFAULT false, admin boolean NOT NULL DEFAULT false, created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS sessions (
 token text PRIMARY KEY, user_id uuid NOT NULL REFERENCES users ON DELETE CASCADE, csrf text NOT NULL, expires_at timestamptz NOT NULL
);
CREATE TABLE IF NOT EXISTS organizations (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), name text NOT NULL, logo text NOT NULL DEFAULT '',
 status text NOT NULL DEFAULT 'draft' CHECK(status IN ('draft','pending','approved','rejected','suspended')),
 reason text NOT NULL DEFAULT '', created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS memberships (
 organization_id uuid REFERENCES organizations ON DELETE CASCADE, user_id uuid REFERENCES users ON DELETE CASCADE,
 role text NOT NULL CHECK(role IN ('owner','editor')), PRIMARY KEY(organization_id,user_id)
);
CREATE TABLE IF NOT EXISTS tokens (
 token text PRIMARY KEY, user_id uuid REFERENCES users ON DELETE CASCADE, email text NOT NULL DEFAULT '',
 kind text NOT NULL CHECK(kind IN ('verify','reset','invite')), organization_id uuid REFERENCES organizations ON DELETE CASCADE,
 expires_at timestamptz NOT NULL
);
CREATE TABLE IF NOT EXISTS categories (slug text PRIMARY KEY, name text NOT NULL, aliases text[] NOT NULL DEFAULT '{}');
CREATE TABLE IF NOT EXISTS cards (slug text PRIMARY KEY, name text NOT NULL);
CREATE TABLE IF NOT EXISTS cities (slug text PRIMARY KEY, name text NOT NULL, region text NOT NULL, aliases text[] NOT NULL DEFAULT '{}', lat double precision NOT NULL, lng double precision NOT NULL);
INSERT INTO categories VALUES ('boks','Boks',ARRAY['boxing']),('pilates','Pilates','{}'),('joga','Joga',ARRAY['yoga']),('silownia','Siłownia',ARRAY['fitness','trening silowy']),('crossfit','CrossFit','{}'),('taniec','Taniec','{}'),('plywanie','Pływanie','{}'),('sztuki-walki','Sztuki walki',ARRAY['mma','karate']) ON CONFLICT DO NOTHING;
INSERT INTO cards VALUES ('multisport','MultiSport'),('medicover-sport','Medicover Sport'),('fitprofit','FitProfit') ON CONFLICT DO NOTHING;
INSERT INTO cities VALUES ('poznan','Poznań','wielkopolskie',ARRAY['poznan'],52.4064,16.9252),('konin','Konin','wielkopolskie','{}',52.223,18.251),('warszawa','Warszawa','mazowieckie',ARRAY['warsaw'],52.2297,21.0122),('krakow','Kraków','małopolskie',ARRAY['krakow'],50.0647,19.945),('wroclaw','Wrocław','dolnośląskie',ARRAY['wroclaw'],51.1079,17.0385),('gdansk','Gdańsk','pomorskie',ARRAY['gdansk'],54.352,18.6466),('lodz','Łódź','łódzkie',ARRAY['lodz'],51.7592,19.456),('katowice','Katowice','śląskie','{}',50.2649,19.0238) ON CONFLICT DO NOTHING;
CREATE TABLE IF NOT EXISTS locations (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), organization_id uuid NOT NULL REFERENCES organizations ON DELETE CASCADE,
 name text NOT NULL, city text NOT NULL REFERENCES cities(slug), address text NOT NULL, logo text NOT NULL DEFAULT '',
 point geography(Point,4326) NOT NULL, hidden boolean NOT NULL DEFAULT false
);
CREATE INDEX IF NOT EXISTS locations_point_idx ON locations USING gist(point);
CREATE INDEX IF NOT EXISTS locations_city_idx ON locations(city) WHERE NOT hidden;
CREATE INDEX IF NOT EXISTS locations_org_idx ON locations(organization_id);
CREATE TABLE IF NOT EXISTS trainings (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), location_id uuid NOT NULL REFERENCES locations ON DELETE CASCADE,
 name text NOT NULL, category text NOT NULL REFERENCES categories(slug), description text NOT NULL DEFAULT '',
 price integer CHECK(price >= 0), cards text[] NOT NULL DEFAULT '{}', photos text[] NOT NULL DEFAULT '{}',
 signup_url text NOT NULL DEFAULT '', hidden boolean NOT NULL DEFAULT false
);
CREATE INDEX IF NOT EXISTS trainings_location_idx ON trainings(location_id);
CREATE INDEX IF NOT EXISTS trainings_category_idx ON trainings(category) WHERE NOT hidden;
CREATE INDEX IF NOT EXISTS trainings_cards_idx ON trainings USING gin(cards);
CREATE TABLE IF NOT EXISTS series (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), training_id uuid NOT NULL REFERENCES trainings ON DELETE CASCADE,
 start_date date NOT NULL, end_date date, weekdays integer[] NOT NULL, local_time text NOT NULL,
 duration integer NOT NULL CHECK(duration BETWEEN 5 AND 1440), hidden boolean NOT NULL DEFAULT false,
 CHECK(end_date IS NULL OR end_date>=start_date)
);
CREATE TABLE IF NOT EXISTS occurrences (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), training_id uuid NOT NULL REFERENCES trainings ON DELETE CASCADE,
 series_id uuid REFERENCES series ON DELETE CASCADE, local_date date NOT NULL, starts_at timestamptz NOT NULL,
 ends_at timestamptz NOT NULL, hidden boolean NOT NULL DEFAULT false, overridden boolean NOT NULL DEFAULT false,
 UNIQUE(series_id,local_date), CHECK(ends_at > starts_at)
);
CREATE INDEX IF NOT EXISTS occurrences_upcoming_idx ON occurrences(training_id,starts_at) WHERE NOT hidden;
CREATE INDEX IF NOT EXISTS occurrences_time_idx ON occurrences(starts_at) WHERE NOT hidden;
CREATE TABLE IF NOT EXISTS schedule_warnings (series_id uuid REFERENCES series ON DELETE CASCADE, local_date date, message text NOT NULL, PRIMARY KEY(series_id,local_date));
CREATE TABLE IF NOT EXISTS jobs (
 id bigserial PRIMARY KEY, kind text NOT NULL, payload jsonb NOT NULL, dedupe text UNIQUE,
 status text NOT NULL DEFAULT 'pending' CHECK(status IN ('pending','running','done','failed')),
 attempts integer NOT NULL DEFAULT 0, run_at timestamptz NOT NULL DEFAULT now(), locked_at timestamptz, error text NOT NULL DEFAULT '', created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS jobs_pending_idx ON jobs(run_at) WHERE status='pending';
CREATE TABLE IF NOT EXISTS media (
 id uuid PRIMARY KEY DEFAULT gen_random_uuid(), organization_id uuid NOT NULL REFERENCES organizations ON DELETE CASCADE,
 status text NOT NULL DEFAULT 'pending', url text NOT NULL DEFAULT '', thumbnail text NOT NULL DEFAULT '', error text NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS audit_log (id bigserial PRIMARY KEY, actor uuid REFERENCES users, organization_id uuid REFERENCES organizations, action text NOT NULL, created_at timestamptz NOT NULL DEFAULT now());
CREATE INDEX IF NOT EXISTS occurrences_weekday_idx ON occurrences(training_id,(extract(dow FROM starts_at AT TIME ZONE 'Europe/Warsaw')),starts_at) WHERE NOT hidden;
CREATE TABLE IF NOT EXISTS public_revision (singleton boolean PRIMARY KEY DEFAULT true CHECK(singleton), version bigint NOT NULL DEFAULT 1);
INSERT INTO public_revision VALUES(true,1) ON CONFLICT DO NOTHING;
CREATE OR REPLACE FUNCTION bump_public_revision() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 UPDATE public_revision SET version=version+1 WHERE singleton;
 RETURN NULL;
END $$;
DO $$ DECLARE tbl text; BEGIN
 FOREACH tbl IN ARRAY ARRAY['organizations','locations','trainings','series','occurrences','categories','cards','cities'] LOOP
  IF NOT EXISTS(SELECT 1 FROM pg_trigger WHERE tgname='public_revision_'||tbl) THEN
   EXECUTE format('CREATE TRIGGER %I AFTER INSERT OR UPDATE OR DELETE ON %I FOR EACH STATEMENT EXECUTE FUNCTION bump_public_revision()', 'public_revision_'||tbl,tbl);
  END IF;
 END LOOP;
END $$;
