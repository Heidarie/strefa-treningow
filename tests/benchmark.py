#!/usr/bin/env python3
"""Fixed-arrival load test without additional dependencies; use isolated fixtures."""
import concurrent.futures,json,os,statistics,time,urllib.request
base=os.environ.get('BASE_URL','http://localhost:18081')
rate=20;seconds=int(os.environ.get('DURATION','60'))
paths=['/api/v1/search?city=poznan&category=pilates&day=1','/api/v1/map?city=poznan&bbox=16.7,52.2,17.2,52.6&zoom=11']
def request(n):
 start=time.perf_counter()
 try:
  with urllib.request.urlopen(base+paths[n%2],timeout=15) as r:r.read();code=r.status
 except Exception:code=0
 return (time.perf_counter()-start)*1000,code
for i in range(20):request(i)
with concurrent.futures.ThreadPoolExecutor(max_workers=80) as pool:
 start=time.perf_counter();futures=[]
 for n in range(rate*seconds):
  time.sleep(max(0,start+n/rate-time.perf_counter()));futures.append(pool.submit(request,n))
 results=[f.result() for f in futures]
latencies=sorted(x[0] for x in results)
report={'requests':len(results),'rate':rate,'duration_seconds':seconds,'p50_ms':round(statistics.median(latencies),2),'p95_ms':round(latencies[int(len(latencies)*.95)-1],2),'errors':sum(code!=200 for _,code in results),'environment':'Local Docker; not the reference production VPS'}
print(json.dumps(report,indent=2))
assert report['errors']==0
