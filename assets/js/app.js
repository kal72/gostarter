// GoStarter — Frontend Application Logic

document.addEventListener('DOMContentLoaded', () => {
  // Form Elements
  const groupInput = document.getElementById('groupInput');
  const serviceInput = document.getElementById('serviceInput');
  const moduleInput = document.getElementById('moduleInput');
  const httpPort = document.getElementById('httpPort');
  const grpcPort = document.getElementById('grpcPort');
  const httpPortGroup = document.getElementById('httpPortGroup');
  const grpcPortGroup = document.getElementById('grpcPortGroup');

  const protocolCards = document.querySelectorAll('.protocol-card');
  const protocolRadios = document.querySelectorAll('input[name="protocolRadio"]');

  const goVersionInput = document.getElementById('goVersionInput');

  // Extensions Catalog & Selection Management (Spring Initializr Style)
  let EXTENSIONS_CATALOG = [];
  let metadata = null;
  let metadataRevision=0;
  let formDirty = false;
  let applyingDefaults=false;
  let outputBusy = false;
  let lastResolutionKey='';
  const escapeHTML = value => String(value ?? '').replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));

  // Extension Elements
  const selectedExtensionsContainer = document.getElementById('selectedExtensionsContainer');
  const emptyExtensionsPlaceholder = document.getElementById('emptyExtensionsPlaceholder');
  const selectedCountBadge = document.getElementById('selectedCountBadge');
  const btnOpenExtensionsModal = document.getElementById('btnOpenExtensionsModal');
  const btnEmptyAddExtensions = document.getElementById('btnEmptyAddExtensions');

  const extensionsDialog = document.getElementById('extensionsDialog');
  const btnCloseExtensions = document.getElementById('btnCloseExtensions');
  const btnDoneExtensions = document.getElementById('btnDoneExtensions');
  const extSearchInput = document.getElementById('extSearchInput');
  const modalExtensionsList = document.getElementById('modalExtensionsList');
  const modalSelectedCount = document.getElementById('modalSelectedCount');
  const btnSelectAllExt = document.getElementById('btnSelectAllExt');
  const btnClearAllExt = document.getElementById('btnClearAllExt');
  const categoryPills = document.querySelectorAll('.ext-cat-pill');

  // Active Extensions State (Default: empty per user review requirement)
  const selectedExtensions = new Set();
  let currentExtCategory = 'all';
  let extSearchQuery = '';

  function resolvedExtensions() { return GoStarterState.resolveExtensions(EXTENSIONS_CATALOG, selectedExtensions); }
  function renderSelectedExtensions() {
    const resolved = resolvedExtensions();
    const active = new Map(resolved.map(e => [e.id,e]));
    for (const id of selectedExtensions) if (!active.has(id)) active.set(id,{id,selected_by_user:true,locked:false,required_by:[]});
    selectedCountBadge.textContent = `${selectedExtensions.size} selected · ${[...active.values()].filter(e=>!selectedExtensions.has(e.id)).length} dependencies`;
    selectedExtensionsContainer.replaceChildren();
    selectedExtensionsContainer.classList.toggle('hidden', active.size===0);
    emptyExtensionsPlaceholder.classList.toggle('hidden', active.size!==0);
    for (const ext of EXTENSIONS_CATALOG) {
      const decision=active.get(ext.id); if(!decision) continue;
      const card=document.createElement('div'); card.className='p-3 rounded-xl border border-slate-300 dark:border-dark-border'; card.dataset.extension=ext.id;
      card.innerHTML=`<strong>${escapeHTML(ext.name)}</strong> <small>${escapeHTML(ext.path)}</small><p class="text-xs">${escapeHTML(ext.description)}</p><p class="text-xs">${decision.locked?'Locked · required by '+escapeHTML(decision.required_by.join(', ')):selectedExtensions.has(ext.id)?'Selected by you':'Automatic dependency'}</p><button type="button" class="btn-remove-ext text-xs text-brand-600" ${decision.locked?'disabled':''}>${decision.locked?'Locked':'Remove'}</button>`;
      card.querySelector('button').addEventListener('click',()=>toggleExtension(ext.id)); selectedExtensionsContainer.append(card);
    }
  }
  function renderModalExtensions() {
    modalSelectedCount.textContent = selectedExtensions.size;
    if (!metadata || !extensionsDialog.open) return;
    const decisions = new Map(resolvedExtensions().map(e=>[e.id,e]));
    const grid = document.createElement('div'); grid.className='grid grid-cols-1 sm:grid-cols-2 gap-3';
    const search=extSearchQuery.toLowerCase();
    for (const ext of EXTENSIONS_CATALOG) {
      if(currentExtCategory!=='all' && ext.kind!==currentExtCategory)continue;
      if(!`${ext.id} ${ext.name} ${ext.description} ${ext.path}`.toLowerCase().includes(search))continue;
      const d=decisions.get(ext.id), active=!!d, locked=!!d?.locked, selected=selectedExtensions.has(ext.id);
      const card=document.createElement('div');
      card.className='p-3.5 rounded-xl border cursor-pointer transition-all flex flex-col justify-between '+(active?'border-brand-500 bg-brand-500/5 dark:bg-brand-500/10 ring-1 ring-brand-500/30':'border-slate-200 dark:border-dark-border bg-slate-50/40 dark:bg-dark-surface/40 hover:bg-slate-100 dark:hover:bg-dark-surface hover:border-slate-300 dark:hover:border-slate-700');
      card.dataset.extension=ext.id;
      card.innerHTML=`<div><div class="flex items-center justify-between gap-2 mb-1.5"><span class="text-xs font-semibold text-slate-900 dark:text-white">${escapeHTML(ext.name)}</span><span class="font-mono text-[9px] px-1.5 py-0.5 rounded bg-brand-500/10 text-brand-500 border border-brand-500/20">${escapeHTML(ext.path)}</span></div><p class="text-[11px] text-slate-500 dark:text-slate-400 leading-snug mb-3">${escapeHTML(ext.description)}</p></div>${locked?`<p class="text-[10px] text-slate-500 dark:text-slate-400 mb-2">Required by ${escapeHTML(d.required_by.join(', '))}</p>`:''}<div class="flex items-center justify-between pt-2 border-t border-slate-100 dark:border-dark-border/60"><span class="text-[10px] text-slate-400 uppercase tracking-wider font-semibold">${ext.kind==='feature'?'Features':'Packages'}</span><button type="button" ${locked&&selected?'disabled':''} class="btn-toggle-ext px-2.5 py-1 rounded-lg text-xs font-semibold transition-all ${active?'bg-brand-500 text-slate-950 font-bold shadow-sm':'border border-slate-300 dark:border-dark-border text-slate-700 dark:text-slate-300 hover:bg-slate-200 dark:hover:bg-dark-hover'}">${locked?(selected?'Locked':'Keep explicitly'):(selected?'✓ Added':'+ Add')}</button></div>`;
      card.addEventListener('click',()=>{ if(locked&&selected)return; toggleExtension(ext.id); }); grid.append(card);
    }
    if(!grid.children.length) { const empty=document.createElement('p'); empty.className='text-center py-12 text-sm text-slate-400'; empty.textContent='No matching extensions found'; modalExtensionsList.replaceChildren(empty); }
    else modalExtensionsList.replaceChildren(grid);
  }
  function selectionChanged() { formDirty=true; invalidatePlan(); }
  function toggleExtension(id) {
    if(resolvedExtensions().find(e=>e.id===id)?.locked && selectedExtensions.has(id)) return;
    if(selectedExtensions.has(id)) selectedExtensions.delete(id);else selectedExtensions.add(id);
    selectionChanged();
  }

  function openExtensionsModal() {
    if (!extensionsDialog) return;
    if (extSearchInput) extSearchInput.value = '';
    extSearchQuery = '';
    currentExtCategory = 'all';
    categoryPills.forEach(p => {
      if (p.dataset.cat === 'all') {
        p.classList.add('active', 'bg-brand-500', 'text-slate-950', 'font-semibold');
        p.classList.remove('text-slate-600', 'dark:text-slate-300');
      } else {
        p.classList.remove('active', 'bg-brand-500', 'text-slate-950', 'font-semibold');
        p.classList.add('text-slate-600', 'dark:text-slate-300');
      }
    });
    extensionsDialog.showModal();
    renderModalExtensions();
    if (extSearchInput) setTimeout(() => extSearchInput.focus(), 50);
  }

  if (btnOpenExtensionsModal) btnOpenExtensionsModal.addEventListener('click', openExtensionsModal);
  if (btnEmptyAddExtensions) btnEmptyAddExtensions.addEventListener('click', openExtensionsModal);
  if (btnCloseExtensions) btnCloseExtensions.addEventListener('click', () => extensionsDialog.close());
  if (btnDoneExtensions) btnDoneExtensions.addEventListener('click', () => extensionsDialog.close());

  // Category filter pills
  categoryPills.forEach(pill => {
    pill.addEventListener('click', () => {
      currentExtCategory = pill.dataset.cat;
      categoryPills.forEach(p => {
        p.classList.remove('active', 'bg-brand-500', 'text-slate-950', 'font-semibold');
        p.classList.add('text-slate-600', 'dark:text-slate-300');
      });
      pill.classList.add('active', 'bg-brand-500', 'text-slate-950', 'font-semibold');
      pill.classList.remove('text-slate-600', 'dark:text-slate-300');
      renderModalExtensions();
    });
  });

  // Search input filter
  if (extSearchInput) {
    extSearchInput.addEventListener('input', (e) => {
      extSearchQuery = e.target.value.trim();
      renderModalExtensions();
    });
  }

  // Select All & Clear All
  if (btnSelectAllExt) {
    btnSelectAllExt.addEventListener('click', () => {
      EXTENSIONS_CATALOG.forEach(e => selectedExtensions.add(e.id));
      renderModalExtensions();
      selectionChanged();
    });
  }

  if (btnClearAllExt) {
    btnClearAllExt.addEventListener('click', () => {
      selectedExtensions.clear();
      selectionChanged();
    });
  }

  // Floating Action Dock Elements
  const dockModuleText = document.getElementById('dockModuleText');
  const dockGoVersion = document.getElementById('dockGoVersion');
  const dockProtocolTag = document.getElementById('dockProtocolTag');

  const btnGenerate = document.getElementById('btnGenerate');
  const btnExplore = document.getElementById('btnExplore');
  const btnCurl = document.getElementById('btnCurl');

  // Modals
  const exploreDialog = document.getElementById('exploreDialog');
  const btnCloseExplore = document.getElementById('btnCloseExplore');
  const btnModalDownload = document.getElementById('btnModalDownload');
  const treeContainer = document.getElementById('treeContainer');
  const codeViewer = document.getElementById('codeViewer');
  const currentFilePath = document.getElementById('currentFilePath');
  const btnCopyCode = document.getElementById('btnCopyCode');
  const copyCodeText = document.getElementById('copyCodeText');
  const totalFilesCount = document.getElementById('totalFilesCount');

  const curlDialog = document.getElementById('curlDialog');
  const btnCloseCurl = document.getElementById('btnCloseCurl');
  const curlCommandCode = document.getElementById('curlCommandCode');
  const curlExtBadge = document.getElementById('curlExtBadge');
  const btnCopyCurl = document.getElementById('btnCopyCurl');
  const copyCurlText = document.getElementById('copyCurlText');

  const toast = document.getElementById('toast');

  let activeFileMap = {};
  let previewAbort;
  let previewRevision=0;
  const previewCache=new GoStarterState.PreviewCache();
  let sourceGoVersion = '';
  let projectVersion = '0.1.0';
  let currentProtocol = 'dual';

  const controller = new GoStarterState.PlanState((...args)=>fetch(...args), state => {
    const needsRetry = ['failed', 'expired', 'incompatible'].includes(state.status);
    document.getElementById('projectError').classList.toggle('hidden', !needsRetry);
    if(needsRetry) document.getElementById('projectErrorMessage').textContent=state.message || 'We couldn’t prepare your project. Please try again.';
    const key=JSON.stringify([state.plan?.resolved||null,[...selectedExtensions].sort()]);
    if(key!==lastResolutionKey){lastResolutionKey=key;renderSelectedExtensions();renderModalExtensions();}
    updateActions();
  });
  function updateActions() {
    const ready=!!metadata&&!outputBusy;
    btnGenerate.disabled=!ready;btnExplore.disabled=!ready;btnModalDownload.disabled=!ready;
    btnCurl.disabled=!metadata;btnOpenExtensionsModal.disabled=!metadata;
  }
  function invalidatePlan() {
    previewRevision++;previewAbort?.abort();controller.invalidate();activeFileMap={};exploreDialog.close();
  }
  document.getElementById('btnRetryProject').addEventListener('click',()=>metadata?invalidatePlan():fetchInfo());
  updateDock(); updateActions(); fetchInfo();
  const urlParams=new URLSearchParams(window.location.search);

  // Input Event Listeners
  groupInput.addEventListener('input', () => {
    updateCalculatedModule();
    updateDock();
  });

  serviceInput.addEventListener('input', () => {
    updateCalculatedModule();
    updateDock();
  });

  function updateCalculatedModule() {
    const group = groupInput.value.trim().replace(/\/+$/, '');
    const service = serviceInput.value.trim().replace(/^\/+/, '');
    moduleInput.value = group && service ? `${group}/${service}` : service || group;
  }

  function updateDock() {
    const mod = moduleInput.value.trim() || 'github.com/example/my-service';
    dockModuleText.textContent = mod;
  }

  for(const input of [groupInput,serviceInput,httpPort,grpcPort,...protocolRadios]) {
    input.addEventListener(input.type==='radio'||input.tagName==='SELECT'?'change':'input',event=>{if(!applyingDefaults)formDirty=true;invalidatePlan();});
  }

  // Protocol / Transport Selection (Item 5 Review)
  protocolRadios.forEach(radio => {
    radio.addEventListener('change', () => {
      currentProtocol = radio.value;
      protocolCards.forEach(card => {
        card.classList.remove('active', 'border-brand-500', 'bg-brand-500/5', 'dark:bg-brand-500/10', 'ring-2', 'ring-brand-500/30');
      });

      const selectedCard = radio.closest('.protocol-card');
      if (selectedCard) {
        selectedCard.classList.add('active', 'border-brand-500', 'bg-brand-500/5', 'dark:bg-brand-500/10', 'ring-2', 'ring-brand-500/30');
      }

      // Update port visibility based on protocol
      if (currentProtocol === 'grpc') {
        dockProtocolTag.textContent = 'gRPC';
        dockProtocolTag.className = 'px-2 py-0.5 rounded font-mono text-[11px] bg-purple-500/10 text-purple-600 dark:text-purple-400 font-medium';
        grpcPortGroup.style.opacity = '1';
        httpPortGroup.style.opacity = '0.4';
      } else {
        dockProtocolTag.textContent = 'DUAL';
        dockProtocolTag.className = 'px-2 py-0.5 rounded font-mono text-[11px] bg-brand-500/10 text-brand-600 dark:text-brand-400 font-medium';
        grpcPortGroup.style.opacity = '1';
        httpPortGroup.style.opacity = '1';
      }
    });
  });

  function getProjectPayload() {
    return {
      module: moduleInput.value.trim(),
      service_name: serviceInput.value.trim(),
      project_version: projectVersion,
      go_version: sourceGoVersion,
      protocol: currentProtocol,
      http_port: httpPort.value.trim(),
      grpc_port: grpcPort.value.trim(),
      extensions: [...selectedExtensions].sort(),
    };
  }

  // Fetch Server Metadata (Golang version from source)
  function applyDefaults(cfg) {
    applyingDefaults=true;try {
    moduleInput.value=cfg.module;serviceInput.value=cfg.service_name;
    groupInput.value=cfg.module.slice(0,-cfg.service_name.length).replace(/\/$/,'');
    projectVersion=cfg.project_version;httpPort.value=cfg.http_port;grpcPort.value=cfg.grpc_port;
    updateCalculatedModule();
    const radio=[...protocolRadios].find(r=>r.value===cfg.protocol);if(radio){radio.checked=true;radio.dispatchEvent(new Event('change'));}
    updateDock();dockGoVersion.textContent=`Go ${sourceGoVersion}`;
    }finally{applyingDefaults=false;}
  }
  async function fetchInfo() {
    const revision=++metadataRevision;
    try {
      const res=await fetch('/api/v1/info',{cache:'no-cache'});const data=await res.json();
      if(revision!==metadataRevision)return;
      if(!res.ok) throw new Error(data.error?.code==='snapshot_unavailable'
        ? 'Upstream needs validation for the current generator version. Open Admin, request Sync, and wait for validation to succeed.'
        : data.error?.message || data.diagnostic?.cause || 'Catalog unavailable. Please try again.');
      metadata=data;sourceGoVersion=data.source_go_version;
      document.getElementById('catalogTotal').textContent=data.catalog.extensions.length;
      EXTENSIONS_CATALOG=data.catalog.extensions.map(e=>({...e,name:e.title,category:e.kind}));
      goVersionInput.value=sourceGoVersion;
      dockGoVersion.textContent=`Go ${sourceGoVersion}`;
      protocolRadios.forEach(r=>{const supported=data.protocols.some(p=>p.id===r.value);r.disabled=!supported;r.closest('.protocol-card').hidden=!supported;});
      if(!formDirty) applyDefaults(data.default_config);
      const preset=urlParams.get('ext')||urlParams.get('extensions');
      if(preset&&!formDirty)for(const id of preset.split(','))if(EXTENSIONS_CATALOG.some(e=>e.id===id.trim()))selectedExtensions.add(id.trim());
      renderSelectedExtensions();renderModalExtensions();invalidatePlan();
      if(urlParams.get('modal')==='extensions')openExtensionsModal();
    } catch(err) { if(revision!==metadataRevision)return;controller.emit({status:'failed',message:err.message}); }
  }

  // Generate & Download ZIP
  // Resolve and poll first: HTTP 202 is JSON, never a downloadable ZIP.
  async function readyPlan(payload) {
    if(!controller.ready()) await controller.resolve(payload);
    if(JSON.stringify(payload)!==JSON.stringify(getProjectPayload()))throw new Error('Project changed; please try again.');
    if(!controller.ready())throw new Error(controller.state.message || 'Project is unavailable. Please try again.');
    return controller.state.plan_id;
  }
  async function triggerGenerate() {
    if(!metadata||outputBusy)return;
    outputBusy=true;updateActions();
    const payload = getProjectPayload();
    btnGenerate.disabled = true;
    const originalText = btnGenerate.querySelector('span').textContent;
    btnGenerate.querySelector('span').textContent = 'Generating...';

    try {
      const planID = await readyPlan(payload);
      const res = await fetch('/api/v1/generate', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({plan_id: planID})
      });

      if (res.status !== 200 || !res.headers.get('Content-Type')?.startsWith('application/zip')) {
        if([202,409,410,422].includes(res.status))invalidatePlan();
        const text = await res.text();
        let message=text;try{message=JSON.parse(text).error?.message||'Project is pending or unavailable. Resolve again.';}catch{}
        throw new Error(message || 'Generation failed');
      }

      const blob = await res.blob();
      if(JSON.stringify(payload)!==JSON.stringify(getProjectPayload()))throw new Error('Project changed; discard old download');
      const url = window.URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `${payload.service_name || 'project'}.zip`;
      document.body.appendChild(a);
      a.click();
      a.remove();
      window.URL.revokeObjectURL(url);

      showToast(`Downloaded ${a.download}`);
    } catch (err) {
      showToast(`Error: ${err.message}`);
    } finally {
      outputBusy=false;updateActions();
      btnGenerate.querySelector('span').textContent = originalText;
    }
  }

  btnGenerate.addEventListener('click', triggerGenerate);
  btnModalDownload.addEventListener('click', triggerGenerate);

  // Explore Project Modal
  async function triggerExplore() {
    if(!metadata||outputBusy)return;
    outputBusy=true;updateActions();
    const payload = getProjectPayload();
    const revision=++previewRevision;
    previewAbort?.abort();
    const abort=previewAbort=new AbortController();
    const key=JSON.stringify([metadata.snapshot.id,metadata.engine_version,metadata.template_version,payload]);
    btnExplore.disabled = true;
    const originalText = btnExplore.querySelector('span').textContent;
    btnExplore.querySelector('span').textContent = 'Loading...';

    try {
      let data=previewCache.get(key);
      if(!data) {
      const res = await fetch('/api/v1/explore', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
        signal:abort.signal
      });

      if (res.status !== 200) {
        if([202,409,410,422].includes(res.status))invalidatePlan();
        const errText = await res.text();
        throw new Error(errText);
      }

      data = await res.json();
      if(revision!==previewRevision||abort.signal.aborted)return;
      previewCache.set(key,data);
      }
      if(revision!==previewRevision||abort.signal.aborted)return;
      if(JSON.stringify(payload)!==JSON.stringify(getProjectPayload()))throw new Error('Project changed; discard old preview');
      activeFileMap = data.files || {};
      totalFilesCount.textContent = `${data.total_files} files`;
      document.getElementById('exploreSubtitle').textContent = `Target: ${payload.module} (${payload.protocol.toUpperCase()}) · Source preview; generated files finalized on download`;

      renderTree(data.tree);

      // Auto-open go.mod or main.go
      const preferred = ['go.mod', 'cmd/api/main.go', 'bootstrap/app.go', 'Makefile'];
      let opened = false;
      for (const p of preferred) {
        if (activeFileMap[p]) {
          loadFile(p);
          opened = true;
          break;
        }
      }
      if (!opened && Object.keys(activeFileMap).length > 0) {
        loadFile(Object.keys(activeFileMap)[0]);
      }

      exploreDialog.showModal();
    } catch (err) {
      if(err.name!=='AbortError')showToast(`Could not open explorer: ${err.message}`);
    } finally {
      outputBusy=false;updateActions();
      btnExplore.querySelector('span').textContent = originalText;
    }
  }

  btnExplore.addEventListener('click', triggerExplore);
  btnCloseExplore.addEventListener('click', () => {previewAbort?.abort();exploreDialog.close();});
  window.addEventListener('pagehide',()=>{previewRevision++;previewAbort?.abort();controller.abort?.abort();});

  // Render Tree Hierarchy
  function renderTree(rootNode) {
    treeContainer.innerHTML = '';
    if (!rootNode || !rootNode.children) return;

    const list = document.createElement('div');
    renderNodeChildren(rootNode.children, list);
    treeContainer.appendChild(list);
  }

  function renderNodeChildren(children, container) {
    children.forEach(node => {
      const item = document.createElement('div');
      item.className = 'tree-item';

      const label = document.createElement('div');
      label.className = 'tree-node';
      label.dataset.path = node.path;

      const icon = document.createElement('span');
      icon.className = 'node-icon';
      icon.innerHTML = getNodeIcon(node);

      const name = document.createElement('span');
      name.textContent = node.name;

      label.appendChild(icon);
      label.appendChild(name);
      item.appendChild(label);

      if (node.type === 'directory') {
        const childrenDiv = document.createElement('div');
        childrenDiv.className = 'tree-children';
        renderNodeChildren(node.children || [], childrenDiv);
        item.appendChild(childrenDiv);

        label.addEventListener('click', (e) => {
          e.stopPropagation();
          const isCollapsed = childrenDiv.style.display === 'none';
          childrenDiv.style.display = isCollapsed ? 'block' : 'none';
          icon.innerHTML = isCollapsed ? '📂' : '📁';
        });
      } else {
        label.addEventListener('click', (e) => {
          e.stopPropagation();
          document.querySelectorAll('.tree-node').forEach(el => el.classList.remove('active'));
          label.classList.add('active');
          loadFile(node.path);
        });
      }

      container.appendChild(item);
    });
  }

  function getNodeIcon(node) {
    if (node.type === 'directory') return '📁';
    if (node.name.endsWith('.go')) return '🔷';
    if (node.name.endsWith('.proto')) return '⚡';
    if (node.name.endsWith('.mod')) return '📦';
    if (node.name.endsWith('.md')) return '📝';
    if (node.name.endsWith('.yml') || node.name.endsWith('.yaml')) return '⚙️';
    if (node.name.endsWith('.sh') || node.name.endsWith('.bat')) return '💻';
    if (node.name.startsWith('.env')) return '🔒';
    return '📄';
  }

  function loadFile(relPath) {
    currentFilePath.textContent = relPath;
    const content = activeFileMap[relPath] ?? '// File not found or empty';
    codeViewer.textContent = content;

    document.querySelectorAll('.tree-node').forEach(el => {
      if (el.dataset.path === relPath) {
        el.classList.add('active');
      }
    });
  }

  // Copy Code in Explorer
  btnCopyCode.addEventListener('click', () => {
    const text = codeViewer.textContent;
    navigator.clipboard.writeText(text).then(() => {
      copyCodeText.textContent = 'Copied!';
      setTimeout(() => { copyCodeText.textContent = 'Copy'; }, 2000);
    });
  });

  // cURL Dialog (Base64 Encoded Config & Extensions)
  btnCurl.addEventListener('click', () => {
    const p = getProjectPayload();
    const host = window.location.origin;

    const extArray = Array.from(selectedExtensions);
    // Use exactly the same compatibility payload as the JSON API.
    const configObj = p;

    // URL-safe Base64 encoding without padding
    const b64 = btoa(unescape(encodeURIComponent(JSON.stringify(configObj))))
      .replace(/\+/g, '-')
      .replace(/\//g, '_')
      .replace(/=+$/, '');

    const targetService = p.service_name || 'project';
    const downloadUrl = `${host}/starter.zip?config=${b64}`;
    const shellQuote = value => "'" + value.replace(/'/g, "'\\''") + "'";
    const zip = shellQuote(targetService + '.zip');
    const curl = `curl -fL ${shellQuote(downloadUrl)} -o ${zip} &&\n  unzip ${zip} &&\n  rm ${zip}`;

    curlCommandCode.textContent = curl;
    if (curlExtBadge) {
      curlExtBadge.textContent = `${extArray.length} extension${extArray.length === 1 ? '' : 's'}`;
    }

    curlDialog.showModal();
  });

  btnCloseCurl.addEventListener('click', () => curlDialog.close());

  btnCopyCurl.addEventListener('click', () => {
    const cmd = curlCommandCode.textContent;
    navigator.clipboard.writeText(cmd).then(() => {
      copyCurlText.textContent = 'Tersalin!';
      setTimeout(() => { copyCurlText.textContent = 'Salin Perintah cURL'; }, 2500);
    });
  });

  // Click on backdrop to dismiss dialog
  [exploreDialog, curlDialog, extensionsDialog].forEach(dialog => {
    if (!dialog) return;
    dialog.addEventListener('click', (e) => {
      const rect = dialog.getBoundingClientRect();
      const isInDialog = (
        rect.top <= e.clientY && e.clientY <= rect.top + rect.height &&
        rect.left <= e.clientX && e.clientX <= rect.left + rect.width
      );
      if (!isInDialog) dialog.close();
    });
  });

  // Keyboard Shortcuts (Ctrl+Enter to Generate, Ctrl+Space to Explore, Ctrl+B to Add Extensions)
  document.addEventListener('keydown', (e) => {
    const isCtrlOrCmd = e.ctrlKey || e.metaKey;
    if (isCtrlOrCmd && e.key === 'Enter') {
      e.preventDefault();
      triggerGenerate();
    } else if (isCtrlOrCmd && e.code === 'Space') {
      e.preventDefault();
      triggerExplore();
    } else if (isCtrlOrCmd && (e.key === 'b' || e.key === 'B')) {
      e.preventDefault();
      openExtensionsModal();
    }
  });

  // Toast Notification
  function showToast(message) {
    toast.textContent = message;
    toast.classList.add('toast-visible');
    setTimeout(() => {
      toast.classList.remove('toast-visible');
    }, 3200);
  }
});
