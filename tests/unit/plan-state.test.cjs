const {test}=require('node:test');const assert=require('node:assert/strict');const {PlanState}=require('../../assets/js/plan-state.js');
const response=(status,data)=>({status,ok:status>=200&&status<300,json:async()=>data});
const ready=id=>({status:'ready',plan_id:id,expires_at:new Date(Date.now()+60000).toISOString(),plan:{resolved:{extensions:[]}}});
test('a late response cannot restore a previous selection',async()=>{
 let complete;const slow=new Promise(resolve=>complete=resolve);let count=0;
 const controller=new PlanState(async()=>++count===1?slow:response(200,ready('new')),()=>{});
 const old=controller.resolve({extensions:['auth']});await controller.resolve({extensions:['user']});complete(response(200,ready('old')));await old;
 assert.equal(controller.state.plan_id,'new');assert.ok(controller.ready());controller.invalidate();assert.equal(controller.ready(),false);
});
test('polling reaches ready and retains dependency reasons',async()=>{
 let count=0;const state=ready('plan');state.plan.resolved.extensions=[{id:'user',locked:true,required_by:['auth']}];
 const controller=new PlanState(async()=>++count===1?response(202,{status:'validating',status_url:'/api/v1/plans/plan'}):response(200,state),()=>{});
 await controller.resolve({extensions:['auth']});assert.equal(count,2);assert.equal(controller.state.plan.resolved.extensions[0].locked,true);controller.invalidate();
});
test('failed, expired, incompatible and unavailable never enable output',async()=>{
 for(const [code,data] of [[422,{status:'failed'}],[410,{status:'expired'}],[409,ready('old')],[503,{error:{message:'Offline'}}],[200,{...ready('expired'),expires_at:new Date(0).toISOString()}]]){
 const controller=new PlanState(async()=>response(code,data),()=>{});await controller.resolve({});assert.equal(controller.ready(),false);controller.invalidate();
 }
});

test('browser resolves transitive dependencies and locks explicit prerequisites', () => {
  const {resolveExtensions} = require('../../assets/js/plan-state.js');
  const catalog = [{id:'auth',requires:['user']},{id:'user',requires:['crypto']},{id:'crypto'}];
  const result = resolveExtensions(catalog,['auth','user']);
  assert.equal(result.length,3);
  assert.equal(result.find(e=>e.id==='user').selected_by_user,true);
  assert.equal(result.find(e=>e.id==='user').locked,true);
  assert.deepEqual(result.find(e=>e.id==='crypto').required_by,['user']);
  assert.deepEqual(resolveExtensions(catalog,[]),[]);
});

test('preview cache bounds memory, expires entries and uses LRU eviction', () => {
 const {PreviewCache}=require('../../assets/js/plan-state.js');let now=0;
 const cache=new PreviewCache(100,2,10,()=>now);
 cache.set('a',{text:'a'});cache.set('b',{text:'b'});cache.get('a');cache.set('c',{text:'c'});
 assert.equal(cache.get('b'),undefined);assert.equal(cache.get('a').text,'a');
 now=11;assert.equal(cache.get('a'),undefined);
 cache.set('large',{text:'x'.repeat(100)});assert.equal(cache.get('large'),undefined);assert.ok(cache.bytes<=100);
});
