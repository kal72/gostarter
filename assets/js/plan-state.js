/* Shared asynchronous plan state; no catalog or dependency rules live here. */
(function (root) {
  class PlanState {
    constructor(fetcher, changed, clock = () => Date.now()) {
      this.fetcher = fetcher; this.changed = changed; this.clock = clock;
      this.revision = 0; this.state = {status: 'loading'};
    }
    emit(state) { this.state = state; this.changed(state); }
    invalidate() {
      this.revision++; this.abort?.abort(); clearTimeout(this.timer);
      this.emit({status: 'editing'});
    }
    async resolve(payload) {
      this.invalidate(); const revision = this.revision;
      const abort = this.abort = new AbortController();
      const current = () => revision === this.revision && !abort.signal.aborted;
      this.emit({status: 'pending'});
      try {
        let response = await this.fetcher('/api/v1/resolve', {method:'POST', headers:{'Content-Type':'application/json'}, body:JSON.stringify(payload), signal:abort.signal});
        while (current()) {
          const data = await response.json(); if (!current()) return;
          const status = response.status===409 ? 'incompatible' : data.status || 'failed';
          this.emit({...data, status, message:data.error?.message || data.diagnostic || ''});
          if (response.status !== 202) {
            if (!response.ok && !data.status) this.emit({...data,status:'failed',message:data.error?.message || 'Unable to resolve project'});
            if (response.ok && status === 'ready') {
              const delay = Date.parse(data.expires_at) - this.clock();
              if (!(delay > 0)) this.emit({...data,status:'expired'});
              else this.timer = setTimeout(() => {if(current()) this.emit({...data,status:'expired'});}, Math.min(delay,2147483647));
            }
            return;
          }
          if (!data.status_url || !data.status_url.startsWith('/api/v1/plans/')) throw new Error('Missing plan status URL');
          await new Promise((resolve,reject) => {
            const cancel = () => {clearTimeout(timer);reject(new DOMException('Canceled','AbortError'));};
            const timer = setTimeout(() => {abort.signal.removeEventListener('abort',cancel);resolve();},1000);
            abort.signal.addEventListener('abort',cancel,{once:true});
          });
          if (!current()) return;
          response = await this.fetcher(data.status_url,{signal:abort.signal});
        }
      } catch (error) {if(current()) this.emit({status:'failed',message:error.message});}
    }
    ready() { return this.state.status === 'ready' && Date.parse(this.state.expires_at) > this.clock(); }
  }
  // Browser preview only; the server remains the authority at output time.
  function resolveExtensions(catalog, selected) {
    const byID = new Map(catalog.map(e => [e.id,e]));
    const explicit = new Set(selected), active = new Map(), visiting = new Set(), expanded = new Set();
    function visit(id, parent) {
      const ext = byID.get(id);
      if (!ext) throw new Error(`Unknown extension: ${id}`);
      if (visiting.has(id)) throw new Error(`Dependency cycle: ${id}`);
      let decision = active.get(id);
      if (!decision) { decision = {id, selected_by_user:explicit.has(id), locked:false, required_by:[]}; active.set(id,decision); }
      if (parent && !decision.required_by.includes(parent)) decision.required_by.push(parent);
      decision.locked = decision.required_by.length > 0;
      if (expanded.has(id)) return;
      visiting.add(id);
      for (const dep of ext.requires || []) visit(dep,id);
      visiting.delete(id); expanded.add(id);
    }
    for (const id of explicit) visit(id);
    return [...active.values()];
  }
  class PreviewCache {
    constructor(maxBytes=8*1024*1024, maxEntries=3, ttl=120000, clock=()=>Date.now()) {
      this.maxBytes=maxBytes; this.maxEntries=maxEntries; this.ttl=ttl; this.clock=clock; this.entries=new Map(); this.bytes=0;
    }
    get(key) {
      const entry=this.entries.get(key); if(!entry)return;
      if(entry.until<=this.clock()){this.remove(key);return;}
      this.entries.delete(key);this.entries.set(key,entry);return entry.value;
    }
    remove(key) { const entry=this.entries.get(key); if(entry){this.bytes-=entry.bytes;this.entries.delete(key);} }
    set(key,value) {
      this.remove(key);const bytes=JSON.stringify(value).length*2;
      if(bytes>this.maxBytes)return;
      this.entries.set(key,{value,bytes,until:this.clock()+this.ttl});this.bytes+=bytes;
      while(this.bytes>this.maxBytes||this.entries.size>this.maxEntries)this.remove(this.entries.keys().next().value);
    }
  }
  if (typeof module !== 'undefined') module.exports = {PlanState, resolveExtensions, PreviewCache};
  else root.GoStarterState = {PlanState, resolveExtensions, PreviewCache};
})(globalThis);
