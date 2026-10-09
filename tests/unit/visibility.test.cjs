const {test}=require('node:test');const assert=require('node:assert/strict');const vm=require('node:vm');const fs=require('node:fs');
test('hidden admin tabs skip status polling and refresh when visible',()=>{
 const events={};let refreshes=0;const document={hidden:true,body:{},addEventListener:(name,fn)=>events[name]=fn,getElementById:()=>({})};
 vm.runInNewContext(fs.readFileSync('assets/js/ui.js','utf8'),{document,htmx:{trigger:()=>refreshes++}});
 let prevented=0;
 events['htmx:beforeRequest']({detail:{requestConfig:{path:'/ui/admin/status'}},preventDefault:()=>prevented++});
 events['htmx:beforeRequest']({detail:{requestConfig:{path:'/ui/admin/settings'}},preventDefault:()=>prevented++});
 assert.equal(prevented,1);events.visibilitychange();assert.equal(refreshes,0);
 document.hidden=false;events.visibilitychange();assert.equal(refreshes,1);
});
