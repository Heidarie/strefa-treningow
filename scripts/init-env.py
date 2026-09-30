#!/usr/bin/env python3
from pathlib import Path
import secrets
p=Path(__file__).resolve().parents[1]/'.env'
if p.exists():
    print('.env already exists; left unchanged')
else:
    s=(p.parent/'.env.example').read_text().replace('replace-with-generated-seed',secrets.token_hex(16))
    p.write_text(s)
    p.chmod(0o600)
    print('Created .env. Set real credentials before production.')
