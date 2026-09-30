#!/usr/bin/env python3
from pathlib import Path
from urllib.parse import urlparse
values=dict(line.split('=',1) for line in (Path(__file__).resolve().parents[1]/'.env').read_text().splitlines() if line and not line.startswith('#') and '=' in line)
errors=[]
url=urlparse(values.get('PUBLIC_URL',''))
if url.scheme!='https' or url.hostname in ('localhost',None):errors.append('PUBLIC_URL must use the real HTTPS domain')
if values.get('DOMAIN')!=url.hostname:errors.append('DOMAIN must match PUBLIC_URL')
for key in ['POSTGRES_PASSWORD','S3_SECRET_KEY','ADMIN_PASSWORD','GRAFANA_ADMIN_PASSWORD','RESTIC_PASSWORD']:
 value=values.get(key,'')
 if len(value)<24 or 'local-' in value or 'change-me' in value:errors.append(key+' must be replaced with a strong production secret')
for key in ['MAPTILER_KEY','RESTIC_REPOSITORY','SMTP_FROM','ALERT_EMAIL']:
 if not values.get(key):errors.append(key+' is required')
if values.get('SMTP_HOST') in ('mailpit','localhost',''):errors.append('Configure production SMTP')
if values.get('SMTP_REQUIRE_TLS')!='true':errors.append('SMTP_REQUIRE_TLS must be true')
if values.get('SEED_DEMO')!='false':errors.append('SEED_DEMO must be false')
if errors:
 print('\n'.join('- '+e for e in errors));raise SystemExit(1)
print('Production environment checks passed. Also configure Alertmanager receiver and off-server backups.')
