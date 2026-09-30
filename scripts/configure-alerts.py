#!/usr/bin/env python3
"""Write a private Alertmanager config from deployment environment; sends no email."""
import json
from pathlib import Path
root=Path(__file__).resolve().parents[1]
env=dict(line.split('=',1) for line in (root/'.env').read_text().splitlines() if line and not line.startswith('#') and '=' in line)
if not env.get('ALERT_EMAIL'):raise SystemExit('Set ALERT_EMAIL first')
global_config={'resolve_timeout':'5m','smtp_smarthost':env['SMTP_HOST']+':'+env['SMTP_PORT'],'smtp_from':env['SMTP_FROM'],'smtp_require_tls':env.get('SMTP_REQUIRE_TLS')=='true'}
if env.get('SMTP_USER'):
 global_config.update(smtp_auth_username=env['SMTP_USER'],smtp_auth_password=env.get('SMTP_PASSWORD',''))
config={'global':global_config,'route':{'receiver':'operations','group_by':['alertname'],'group_wait':'30s','repeat_interval':'4h'},'receivers':[{'name':'operations','email_configs':[{'to':env['ALERT_EMAIL'],'send_resolved':True}]}]}
p=root/'infra/observability/alertmanager.generated.json';p.write_text(json.dumps(config,indent=2)+'\n');p.chmod(0o600)
print('Wrote private Alertmanager configuration. No email sent.')
