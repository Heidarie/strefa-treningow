// k6 run tests/load.js -e BASE_URL=http://localhost:8080
import http from 'k6/http';
import { check } from 'k6';
export const options={scenarios:{search:{executor:'constant-arrival-rate',rate:20,timeUnit:'1s',duration:'2m',preAllocatedVUs:20,maxVUs:80}},thresholds:{http_req_duration:['p(95)<300'],http_req_failed:['rate<0.01']}};
const base=__ENV.BASE_URL||'http://localhost:8080';
export default function(){const path=__ITER%2===0?'/api/v1/search?city=poznan&category=pilates&day=1':'/api/v1/map?city=poznan&bbox=16.7,52.2,17.2,52.6&zoom=11';const r=http.get(base+path);check(r,{'200':v=>v.status===200});}
