#!/usr/bin/env python3
"""Live local acceptance test; uses Mailpit, never an external mailbox.
Requires the demo admin configured in .env. Creates a uniquely named test organization,
then suspends it at the end. No external booking request is sent.
"""
import base64, http.cookiejar, json, os, re, time, urllib.request, urllib.error, uuid, struct, zlib
from pathlib import Path
ROOT=Path(__file__).resolve().parents[1]
env=dict(line.split('=',1) for line in (ROOT/'.env').read_text().splitlines() if line and not line.startswith('#') and '=' in line)
BASE=os.environ.get('API_URL','http://localhost:'+env.get('API_PORT','18080'))+'/api/v1'
WEB=env.get('PUBLIC_URL','http://localhost:3100')
MAIL='http://localhost:8025'
class Client:
 def __init__(self):
  self.opener=urllib.request.build_opener(urllib.request.HTTPCookieProcessor(http.cookiejar.CookieJar()));self.csrf=''
 def call(self,path,body=None,method=None,expected=200,headers=None):
  headers={'Content-Type':'application/json','X-CSRF-Token':self.csrf,**(headers or {})}
  data=body if isinstance(body,bytes) else json.dumps(body).encode() if body is not None else None
  req=urllib.request.Request(BASE+path,data=data,headers=headers,method=method or ('POST' if body is not None else 'GET'))
  try:
   with self.opener.open(req,timeout=15) as r: code=r.status;raw=r.read()
  except urllib.error.HTTPError as e:code=e.code;raw=e.read()
  assert code==expected,(path,code,raw.decode()[:300])
  return json.loads(raw) if raw else None
 def login(self,email,password):
  self.csrf=self.call('/auth/login',{'email':email,'password':password})['csrf']
def token_from_mail(email,kind):
 for _ in range(40):
  with urllib.request.urlopen(MAIL+'/api/v1/messages') as r:messages=json.load(r)['messages']
  for m in messages:
   if any(x['Address']==email for x in m['To']):
    with urllib.request.urlopen(MAIL+'/api/v1/message/'+m['ID']) as r:full=json.load(r)
    found=re.search(r'token=([a-f0-9]+)&kind='+kind,full.get('Text',''))
    if found:return found.group(1)
  time.sleep(.5)
 raise AssertionError('Mailpit message not delivered')
def png():
 def chunk(kind,data):return struct.pack('>I',len(data))+kind+data+struct.pack('>I',zlib.crc32(kind+data)&0xffffffff)
 return b'\x89PNG\r\n\x1a\n'+chunk(b'IHDR',struct.pack('>IIBBBBB',32,32,8,2,0,0,0))+chunk(b'IDAT',zlib.compress((b'\x00'+b'\x33\x77\x44'*32)*32))+chunk(b'IEND',b'')
def register(c,email):
 c.call('/auth/register',{'email':email,'password':'Acceptance-test-password'},expected=201)
 c.call('/auth/token',{'token':token_from_mail(email,'verify'),'password':''})
 c.login(email,'Acceptance-test-password')
admin=Client();admin.login(env['ADMIN_EMAIL'],env['ADMIN_PASSWORD'])
owner=Client();name='Acceptance '+uuid.uuid4().hex[:8];email=name.replace(' ','').lower()+'@example.test';register(owner,email)
org=owner.call('/organizations',{'name':name})['id']
loc=owner.call('/locations',{'organization_id':org,'name':name,'city':'konin','address':'Testowa 1','logo':'','lat':52.22,'lng':18.25,'hidden':False})['id']
training={'location_id':loc,'name':'Trening testowy','category':'boks','description':'Acceptance test fixture','price':None,'cards':['multisport'],'photos':[],'signup_url':'https://example.com/signup','hidden':False}
tid=owner.call('/trainings',training)['id']
public=Client();public.call('/trainings/'+tid,expected=404)
owner.call('/organizations/'+org+'/submit',method='POST')
admin.call('/admin/organizations/'+org,{'status':'approved','reason':''},method='PUT')
public.call('/trainings/'+tid)
series=owner.call('/schedules',{'training_id':tid,'start_date':time.strftime('%Y-%m-%d'),'end_date':'','weekdays':[0,1,2,3,4,5,6],'local_time':'18:00','duration':60,'hidden':False,'one_off':False})['id']
assert len(public.call('/trainings/'+tid)['occurrences'])>80
boundary='strefa'+uuid.uuid4().hex
body=(f'--{boundary}\r\nContent-Disposition: form-data; name="organization_id"\r\n\r\n{org}\r\n--{boundary}\r\nContent-Disposition: form-data; name="file"; filename="test.png"\r\nContent-Type: image/png\r\n\r\n'.encode()+png()+f'\r\n--{boundary}--\r\n'.encode())
mid=owner.call('/media',body,headers={'Content-Type':'multipart/form-data; boundary='+boundary})['id']
for _ in range(40):
 media=next(m for m in owner.call('/panel')['media'] if m['id']==mid)
 if media['status']=='ready':break
 time.sleep(.5)
assert media['status']=='ready',media
training['photos']=[media['url']];owner.call('/trainings/'+tid,training,method='PUT')
with urllib.request.urlopen(WEB+media['url']) as r: assert r.headers['Content-Type']=='image/jpeg' and r.read()[:2]==b'\xff\xd8'
editor=Client();editor_email='editor-'+uuid.uuid4().hex[:8]+'@example.test';register(editor,editor_email)
owner.call('/organizations/'+org+'/invite',{'email':editor_email})
editor.call('/auth/token',{'token':token_from_mail(editor_email,'invite'),'password':''})
editor.call('/trainings/'+tid,training,method='PUT')
editor.call('/organizations/'+org+'/invite',{'email':'another@example.test'},expected=403)
editor_id=editor.call('/auth/me')['id'];owner.call('/organizations/'+org+'/members/'+editor_id,method='DELETE')
editor.call('/trainings/'+tid,training,method='PUT',expected=403)
owner.call('/auth/reset-request',{'email':email})
owner.call('/auth/token',{'token':token_from_mail(email,'reset'),'password':'New-acceptance-password'})
owner.call('/panel',expected=401)
owner.login(email,'New-acceptance-password')
with urllib.request.urlopen(WEB+'/konin/boks') as r:
 html=r.read().decode();assert 'Boks Konin' in html and tid in html and 'rel="canonical"' in html
with urllib.request.urlopen(WEB+'/konin/boks?card=multisport') as r:assert 'noindex,follow' in r.read().decode()
admin.call('/admin/organizations/'+org,{'status':'suspended','reason':'Acceptance test completed; synthetic fixture hidden'},method='PUT')
public.call('/trainings/'+tid,expected=404)
print('PASS: registration, email verification, login, publication, schedules, image processing, editor invitations, role isolation, password reset, SSR, SEO, suspension')
