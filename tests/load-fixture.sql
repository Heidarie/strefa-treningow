-- Use ONLY on an isolated performance database; explicit transaction and unique fixture name.
BEGIN;
DO $$ BEGIN
 IF EXISTS(SELECT 1 FROM organizations WHERE name='LOAD TEST FIXTURE') THEN
  RAISE EXCEPTION 'Load fixture already exists';
 END IF;
END $$;
CREATE TEMP TABLE load_org AS WITH x AS(INSERT INTO organizations(name,status) VALUES('LOAD TEST FIXTURE','approved') RETURNING id) SELECT id FROM x;
INSERT INTO locations(organization_id,name,city,address,point)
SELECT load_org.id,'Load location '||n,'poznan','Testowa '||n,ST_SetSRID(ST_MakePoint(16.7+(n%100)*0.005,52.2+(n/100)*0.004),4326)::geography FROM load_org,generate_series(1,10000) n;
INSERT INTO trainings(location_id,name,category,description,cards)
SELECT l.id,'Load pilates','pilates','Synthetic benchmark data',ARRAY['multisport'] FROM locations l JOIN load_org o ON o.id=l.organization_id;
INSERT INTO occurrences(training_id,local_date,starts_at,ends_at)
SELECT t.id,(CURRENT_DATE+n),CURRENT_DATE+n+time '18:00',CURRENT_DATE+n+time '19:00' FROM trainings t JOIN locations l ON l.id=t.location_id JOIN load_org o ON o.id=l.organization_id CROSS JOIN generate_series(1,50)n;
COMMIT;
ANALYZE;
