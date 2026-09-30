#!/usr/bin/env python3
"""Generate the checked-in OpenAPI contract. Run after changing API shapes."""
import json
from pathlib import Path
ROOT=Path(__file__).resolve().parents[1]
def obj(properties,required=None):
    return {'type':'object','properties':properties,'required':required if required is not None else list(properties),'additionalProperties':False}
def arr(item): return {'type':'array','items':item}
def ref(name): return {'$ref':'#/components/schemas/'+name}
S={'type':'string'};B={'type':'boolean'};I={'type':'integer'};N={'type':'number'};UUID={'type':'string','format':'uuid'};DT={'type':'string','format':'date-time'}
nullable=lambda t: {'anyOf':[t,{'type':'null'}]}
schemas={
 'Error':obj({'error':S}), 'ID':obj({'id':S}), 'OK':obj({'ok':B}), 'Message':obj({'message':S}),
 'Credentials':obj({'email':{'type':'string','format':'email'},'password':{'type':'string','minLength':10,'maxLength':72}}),
 'Session':obj({'id':UUID,'email':S,'admin':B,'csrf':S}),
 'Category':obj({'slug':S,'name':S,'aliases':arr(S)}), 'Card':obj({'slug':S,'name':S}),
 'City':obj({'slug':S,'name':S,'region':S,'aliases':arr(S),'lat':N,'lng':N}),
 'Organization':obj({'id':UUID,'name':S,'logo':S,'status':{'type':'string','enum':['draft','pending','approved','rejected','suspended']},'reason':S,'role':S,'created_at':DT}),
 'LocationInput':obj({'organization_id':UUID,'name':S,'city':S,'address':S,'logo':S,'lat':N,'lng':N,'hidden':B}),
 'TrainingInput':obj({'location_id':UUID,'name':S,'category':S,'description':S,'price':nullable(I),'cards':arr(S),'photos':arr(S),'signup_url':S,'hidden':B}),
 'ScheduleInput':obj({'training_id':UUID,'start_date':{'type':'string','format':'date'},'end_date':S,'weekdays':arr({'type':'integer','minimum':0,'maximum':6}),'local_time':{'type':'string','pattern':'^[0-2][0-9]:[0-5][0-9]$'},'duration':{'type':'integer','minimum':5,'maximum':1440},'hidden':B,'one_off':B}),
 'OccurrenceInput':obj({'date':{'type':'string','format':'date'},'local_time':S,'duration':I,'hidden':B}),
 'Occurrence':obj({'id':UUID,'starts_at':DT,'ends_at':DT}),
 'Training':obj({'id':UUID,'name':S,'category':S,'description':S,'price':nullable(I),'cards':arr(S),'photos':arr(S),'signup_url':S,'next_at':nullable(DT)}),
 'MapPoint':obj({'id':nullable(UUID),'name':S,'count':I,'lng':N,'lat':N}),
 'Media':obj({'id':UUID,'organization_id':UUID,'status':S,'url':S,'thumbnail':S,'error':S}),
 'Job':obj({'id':I,'kind':S,'status':S,'attempts':I,'error':S,'created_at':DT}),
}
schemas['Dictionaries']=obj({'categories':arr(ref('Category')),'cards':arr(ref('Card')),'cities':arr(ref('City'))})
location={'id':UUID,'name':S,'address':S,'city':S,'lat':N,'lng':N,'logo':S}
schemas['Location']=obj({**location,'organization':S,'trainings':arr(ref('Training'))})
schemas['SearchResult']=obj({**schemas['Location']['properties'],'distance_m':nullable(N),'total':I})
schemas['TrainingDetail']=obj({**schemas['Training']['properties'],'organization':S,'location':obj(location),'occurrences':arr(ref('Occurrence'))})
schemas['PanelLocation']=obj({'id':UUID,**schemas['LocationInput']['properties']})
schemas['PanelTraining']=obj({'id':UUID,**schemas['TrainingInput']['properties']})
schemas['Series']=obj({'id':UUID,**{k:v for k,v in schemas['ScheduleInput']['properties'].items() if k!='one_off'}})
schemas['PanelOccurrence']=obj({**schemas['Occurrence']['properties'],'training_id':UUID,'series_id':nullable(UUID),'local_date':S,'hidden':B,'overridden':B})
schemas['Panel']=obj({'organizations':arr(ref('Organization')),'locations':arr(ref('PanelLocation')),'trainings':arr(ref('PanelTraining')),'series':arr(ref('Series')),'occurrences':arr(ref('PanelOccurrence')),'media':arr(ref('Media')),'members':arr(obj({'organization_id':UUID,'user_id':UUID,'email':S,'role':S})),'warnings':arr(obj({'series_id':UUID,'local_date':S,'message':S}))})
schemas['Organization']['required'].remove('role')
schemas['Series']['properties']['end_date']=nullable(S)
paths={}
def endpoint(path,method,summary,response,body=None,auth=False,query=None,status=200):
    op={'summary':summary,'operationId':method+'_'+path.replace('/','_').replace('{','').replace('}','').replace('-','_'),'responses':{str(status):{'description':'Success','content':{'application/json':{'schema':response}}},**{str(n):{'description':d,'content':{'application/json':{'schema':ref('Error')}}} for n,d in [(400,'Invalid input'),(401,'Unauthenticated'),(403,'Forbidden'),(404,'Not found'),(409,'Conflict'),(429,'Rate limited'),(503,'Service unavailable')]}}}
    params=[]
    import re
    for name in re.findall(r'{(.*?)}',path):params.append({'name':name,'in':'path','required':True,'schema':S})
    for name,typ in (query or {}).items():params.append({'name':name,'in':'query','schema':typ})
    if auth:
        op['security']=[{'sessionCookie':[]}]
        if method!='get':params.append({'name':'X-CSRF-Token','in':'header','required':True,'schema':S})
    if params:op['parameters']=params
    if body:op['requestBody']={'required':True,'content':{'application/json':{'schema':body}}}
    paths.setdefault(path,{})[method]=op
filters={k:S for k in ['city','category','card','day','bbox']}
endpoint('/dictionaries','get','Public search dictionaries',ref('Dictionaries'))
endpoint('/search','get','Paginated locations and matching trainings',arr(ref('SearchResult')),query={**filters,'lat':N,'lng':N,'page':I})
endpoint('/map','get','Bounded server-side map clusters',arr(ref('MapPoint')),query={**filters,'zoom':I})
endpoint('/trainings/{id}','get','Published training detail',ref('TrainingDetail'))
endpoint('/locations/{id}','get','Published location detail',ref('Location'))
endpoint('/sitemap','get','Published canonical URLs',arr(obj({'path':S})))
endpoint('/auth/register','post','Create unverified account',ref('Message'),ref('Credentials'),status=201)
endpoint('/auth/login','post','Create HttpOnly session',obj({'csrf':S}),ref('Credentials'))
endpoint('/auth/me','get','Current session and CSRF token',ref('Session'),auth=True)
endpoint('/auth/logout','post','Destroy session',ref('OK'),auth=True)
endpoint('/auth/reset-request','post','Email reset link; always generic response',ref('Message'),obj({'email':S}))
endpoint('/auth/token','post','Consume one-time verification/reset/invitation token',ref('OK'),obj({'token':S,'password':S}))
endpoint('/panel','get','Organization-scoped editor data',ref('Panel'),auth=True)
endpoint('/organizations','post','Create organization and owner membership',ref('ID'),obj({'name':S}),True)
endpoint('/organizations/{id}','put','Owner edits organization',ref('ID'),obj({'name':S,'logo':S}),True)
endpoint('/organizations/{id}/submit','post','Submit initial publication for review',ref('OK'),auth=True)
endpoint('/organizations/{id}/invite','post','Owner invites editor by email',ref('ID'),obj({'email':S}),True)
endpoint('/organizations/{id}/members/{user}','delete','Owner removes editor access',ref('ID'),auth=True)
for path,schema in [('locations','LocationInput'),('trainings','TrainingInput'),('schedules','ScheduleInput')]:
    for suffix,method in [('', 'post'),('/{id}','put')]: endpoint('/'+path+suffix,method,'Save organization-scoped '+path,ref('ID'),ref(schema),True)
endpoint('/occurrences/{id}','put','Override future occurrence while preserving history',ref('ID'),ref('OccurrenceInput'),True)
endpoint('/media','post','Queue validated image upload (JPEG/PNG, 8 MB, 25 MP)',ref('ID'),auth=True)
paths['/media']['post']['requestBody']={'required':True,'content':{'multipart/form-data':{'schema':obj({'organization_id':UUID,'file':{'type':'string','format':'binary'}})}}}
endpoint('/geocode','get','Geocode a Polish address using MapTiler',{'type':'object','additionalProperties':True},auth=True,query={'q':S})
endpoint('/admin','get','Moderation queue and unfinished jobs',obj({'organizations':arr(ref('Organization')),'jobs':arr(ref('Job'))}),auth=True)
endpoint('/admin/organizations/{id}','get','Preview all content before publication',{'type':'object','additionalProperties':True},auth=True)
endpoint('/admin/organizations/{id}','put','Approve, reject or suspend organization',ref('ID'),obj({'status':{'type':'string','enum':['approved','rejected','suspended']},'reason':S}),True)
endpoint('/admin/dictionaries/{kind}','put','Upsert category, card or city',ref('ID'),obj({'slug':S,'name':S,'aliases':arr(S),'region':S,'lat':N,'lng':N}),True)
endpoint('/admin/jobs/{id}/retry','post','Retry failed durable job',ref('ID'),auth=True)
doc={'openapi':'3.1.0','info':{'title':'Strefa Treningów API','version':'1.0.0','description':'Prices are integer PLN grosze; distances meters. bbox=west,south,east,north. day=0 (Sunday)..6. All schedules use Europe/Warsaw.'},'servers':[{'url':'/api/v1'}],'paths':paths,'components':{'schemas':schemas,'securitySchemes':{'sessionCookie':{'type':'apiKey','in':'cookie','name':'strefa_session'}}}}
(ROOT/'openapi.json').write_text(json.dumps(doc,ensure_ascii=False,indent=2)+'\n')
