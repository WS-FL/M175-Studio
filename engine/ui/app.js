'use strict';
const $ = id => document.getElementById(id);
let editingTitle = null;
let trashTarget = null, fileActionBusy = false;
let selectedIDs = new Set(), selectionAnchor = "";
let appState = null, initialized = false, currentTab = 'scan', source = 'platen';
let selectedID = '', pageIndex = 0, rotation = 0, previewSignature = '', listSignature = '', lastCompleted = '';
let requestBusy = false, toastTimer = 0, unavailableCount = 0, lastLog = '', chosenPath = '';
const native = (action, extra={}) => {
  if(window.webkit && window.webkit.messageHandlers && window.webkit.messageHandlers.studio){
    window.webkit.messageHandlers.studio.postMessage({action,...extra});return true;
  }
  return false;
};
function toast(message, error=false){
  clearTimeout(toastTimer);$('toast').textContent=message;$('toast').classList.remove('hidden');$('toast').classList.toggle('error',error);
  toastTimer=setTimeout(()=>$('toast').classList.add('hidden'),error?7000:3300);
}
window.studioToast = message => toast(String(message),true);
window.studioFolderSelected = path => { if(typeof path==='string'){$('settingOutput').value=path;chosenPath=path;toast('Folder selected. Click Save Settings to apply.');} };
window.studioQuitBusy = () => toast('A scan is still running. Wait for it to finish, or click Stop Receiving before quitting.',true);
async function api(path,data){
  const res=await fetch('api/'+path,data===undefined?{}:{method:'POST',headers:{'Content-Type':'application/json','X-M175-Request':'1'},body:JSON.stringify(data)});
  const body=await res.json();if(!res.ok)throw new Error(body.error || 'The operation did not finish');return body;
}
function settingsFromUI(){return {ip:$('settingIP').value.trim(),port:Number($('settingPort').value),output:$('settingOutput').value,source,paper:$('paper').value,dpi:Number($('dpi').value),mode:$('mode').value,format:$('format').value,prefix:$('prefix').value};}
function populateSettings(ss){
  $('settingIP').value=ss.ip;$('settingPort').value=ss.port;$('settingOutput').value=ss.output;
  $('paper').value=ss.paper;$('dpi').value=ss.dpi;$('mode').value=ss.mode;$('format').value=ss.format;$('prefix').value=ss.prefix;setSource(ss.source);updateOptions();
}
function setSource(value){source=value;document.querySelectorAll('[data-source]').forEach(b=>{const sel=b.dataset.source===value;b.classList.toggle('selected',sel);b.setAttribute('aria-pressed',String(sel));});
  $('sourceHelp').textContent=value==='adf'?'Load documents into the feeder. Single-sided ADF scanning still needs hardware testing.':'Place the document face down on the glass.';
}
function updateOptions(){
  const advanced=$('dpi').value!=='300';$('dpiWarning').classList.toggle('hidden',!advanced);
  const gray=$('mode').value==='gray';if(gray)$('format').value='pdf';$('format').disabled=gray||Boolean(appState?.job?.busy);
  $('formatHelp').textContent=gray?'PDF is converted to grayscale locally; original color JPEGs are kept.':$('format').value==='jpeg'?'Save each page as a color JPEG without creating a PDF.':'Combine pages into one PDF; original images are always kept.';
  $('scanFootnote').textContent=source==='adf'?'Single-sided ADF scanning still needs hardware testing.':advanced?'This resolution still needs hardware testing.':'Flatbed color at 300 dpi has been tested on hardware.';
}
const titles={scan:['SCAN WORKSPACE','Scan','Scan, preview and save from your M175nw.'],files:['DOCUMENT LIBRARY','Files','Browse documents saved on this Mac.'],settings:['PREFERENCES','Settings','Manage connection, output and scan preferences.'],diagnostics:['CONNECTION & ACTIVITY','Diagnostics','View scanner status and job logs.']};
function switchTab(tab){
  if(!titles[tab]||trashTarget)return;closeFileMenu();currentTab=tab;
  document.querySelectorAll('[data-tab]').forEach(b=>{let yes=b.dataset.tab===tab;b.classList.toggle('active',yes);b.setAttribute('aria-selected',String(yes));});
  document.querySelectorAll('.page').forEach(p=>{let yes=p.id==='page-'+tab;p.classList.toggle('active',yes);p.hidden=!yes;});
  [$('eyebrow').textContent,$('pageTitle').textContent,$('pageSubtitle').textContent]=titles[tab];
  document.querySelector('.page-header').classList.toggle('scan-header',tab==='scan');
  if(tab==='scan')$('scanPreviewMount').appendChild($('previewPanel'));
  if(tab==='files')$('filesPreviewMount').appendChild($('previewPanel'));
  if(tab==='files'&&!selectedID&&appState?.documents?.length)selectDocument(appState.documents[0].id);
  renderPreview();if(tab==='diagnostics'){$('logText').scrollTop=$('logText').scrollHeight;}
}
window.studioShowSettings = () => switchTab('settings');
function formatDate(s){let d=new Date(s);return isNaN(d)?s:d.toLocaleString('en-US',{month:'2-digit',day:'2-digit',hour:'2-digit',minute:'2-digit',hour12:false});}
function sizeText(bytes){return bytes<1048576?(bytes/1024).toFixed(0)+' KB':(bytes/1048576).toFixed(1)+' MB';}
function asset(doc,name){return 'asset/'+encodeURIComponent(doc.id)+'/'+encodeURIComponent(name);}
function selectDocument(id){selectedIDs=new Set(id?[id]:[]);selectionAnchor=id;previewDocument(id);}
function previewDocument(id){selectedID=id;pageIndex=0;rotation=0;previewSignature='';listSignature='';renderFiles();renderPreview();}
function selectedDocuments(){return visibleDocuments().filter(d=>selectedIDs.has(d.id));}
function selectionPayload(docs){return docs.length===1?{id:docs[0].id}:{ids:docs.map(d=>d.id)};}
function chooseDocument(id,event={}){
  const docs=visibleDocuments(),ids=docs.map(d=>d.id);
  if(event.shiftKey){
    let anchor=ids.indexOf(selectionAnchor);if(anchor<0)anchor=Math.max(0,ids.indexOf(selectedID));
    const index=ids.indexOf(id);if(index<0)return;
    const range=ids.slice(Math.min(anchor,index),Math.max(anchor,index)+1);
    selectedIDs=new Set(event.metaKey?[...selectedIDs,...range]:range);
    selectionAnchor=ids[anchor];previewDocument(id);
  }else if(event.metaKey){
    if(selectedIDs.has(id))selectedIDs.delete(id);else selectedIDs.add(id);
    selectionAnchor=id;previewDocument(selectedIDs.has(id)?id:(docs.find(d=>selectedIDs.has(d.id))?.id||''));
  }else selectDocument(id);
}
function currentDocument(){return appState?.documents?.find(d=>d.id===selectedID)||null;}
function visibleDocuments(){
  const q=$('fileSearch').value.trim().toLocaleLowerCase();
  return (appState?.documents||[]).filter(d=>d.title.toLocaleLowerCase().includes(q));
}
function renderFiles(){
  if(!appState)return;const q=$('fileSearch').value.trim().toLocaleLowerCase();
  const docs=visibleDocuments();
  const count=selectedDocuments().length,blocked=appState.job.busy||fileActionBusy;
  $('renameFile').disabled=count!==1||blocked;
  ['copyFile','trashFile'].forEach(id=>$(id).disabled=count===0||blocked);
  $('copyFile').textContent=count>1?'Copy ('+count+')':'Copy';$('trashFile').textContent=count>1?'Trash ('+count+')…':'Trash…';
  $('selectionCount').textContent=count?count+' selected':'';


  const signature=JSON.stringify([docs,selectedID,[...selectedIDs],q]);if(signature===listSignature)return;listSignature=signature;
  const list=$('documentList');const scroll=list.scrollTop,hadFocus=list.contains(document.activeElement);list.replaceChildren();
  if(!docs.length){let p=document.createElement('div');p.className='empty-list';p.textContent=q?'No matching documents.':'No scans yet.\nYour first scan will appear here.';list.append(p);}
  docs.forEach(doc=>{
    const row=document.createElement('button');row.className='document-item'+(selectedIDs.has(doc.id)?' active':'');row.setAttribute('aria-label',doc.title);row.setAttribute('aria-pressed',String(selectedIDs.has(doc.id)));row.dataset.id=doc.id;row.tabIndex=doc.id===selectedID||!docs.some(d=>d.id===selectedID)?0:-1;
    row.addEventListener('mousedown',e=>{if(e.shiftKey)e.preventDefault();});
    row.addEventListener('click',e=>{chooseDocument(doc.id,e);focusDocument();});
    row.addEventListener('dblclick',beginRename);
    row.addEventListener('contextmenu',e=>{e.preventDefault();if(selectedIDs.has(doc.id))previewDocument(doc.id);else selectDocument(doc.id);showFileMenu(e.clientX,e.clientY);});
    const im=document.createElement('img');im.className='document-thumb';im.src=asset(doc,doc.pages[0]);im.loading='lazy';im.alt='';
    const info=document.createElement('div'),title=document.createElement('strong'),meta=document.createElement('small');title.textContent=doc.title;title.title=doc.title;
    meta.textContent=formatDate(doc.created)+' · '+doc.pages.length+' pages - '+(doc.pdf?'PDF':'JPEG');info.append(title,meta);
    if(doc.partial){let tag=document.createElement('span');tag.className='partial-tag';tag.textContent='Incomplete - received pages kept';info.append(tag);}
    row.append(im,info);list.append(row);
  });list.scrollTop=scroll;if(hadFocus)focusDocument(false);
}
function renderPreview(){
  const doc=currentDocument();const has=Boolean(doc&&doc.pages.length);pageIndex=has?Math.max(0,Math.min(pageIndex,doc.pages.length-1)):0;
  const signature=JSON.stringify([doc?.id,pageIndex,rotation,$('zoom').value,has?doc.pages[pageIndex]:'']);
  $('previewEmpty').classList.toggle('hidden',has);$('previewImage').classList.toggle('hidden',!has);
  $('previewTitle').textContent=has?doc.title:'Page Preview';$('previewTitle').title=has?'Click to rename · '+doc.title:'';
  $('previewTitle').setAttribute('aria-disabled',String(!has||Boolean(appState?.job?.busy)||(currentTab==='files'&&selectedDocuments().length!==1)));
  $('previewTitle').classList.toggle('hidden',Boolean(editingTitle));
  $('previewMeta').textContent=has?doc.paper+' · '+doc.dpi+' dpi · '+(doc.mode==='gray'?'Grayscale':'Color')+' · '+sizeText(doc.bytes):'Available after scanning';
  $('pageNumber').textContent=has?(pageIndex+1)+' / '+doc.pages.length:'0 / 0';
  $('prevPage').disabled=!has||pageIndex===0;$('nextPage').disabled=!has||pageIndex>=doc.pages.length-1;
  $('revealFile').disabled=!has;$('openPDF').disabled=!has;$('rotatePreview').disabled=!has;$('zoom').disabled=!has;
  $('openPDF').lastChild.textContent=has&&!doc.pdf?'Open Image':'Open PDF';
  if(signature!==previewSignature){previewSignature=signature;if(has){$('previewImage').src=asset(doc,doc.pages[pageIndex]);$('previewImage').alt='Page '+(pageIndex+1)+' scan preview';applyZoom();}}
  if(!appState)return;
  const failure=appState.job.stage==='error'||appState.job.stage==='stopped';
  $('previewError').classList.toggle('hidden',!failure);$('previewErrorTitle').textContent=appState.job.message;$('previewErrorHint').textContent=appState.job.hint;
}
function applyZoom(){
  const img=$('previewImage'),z=$('zoom').value,doc=currentDocument();img.style.transform='rotate('+rotation+'deg)';
  $('previewCanvas').classList.toggle('zoomed',z!=='fit');
  if(z==='fit'){img.style.width='';img.style.height='';}else{const natural=img.naturalWidth||850;const screenWidth=natural*96/(doc?.dpi||300);img.style.width=(screenWidth*Number(z)/100)+'px';img.style.height='auto';}
}
function updateElapsed(){
  if(!appState?.job?.started){$('taskTime').textContent='';return;}
  const j=appState.job,end=j.busy?Date.now():new Date(j.ended).getTime(),elapsed=Math.max(0,Math.floor((end-new Date(j.started).getTime())/1000));
  $('taskTime').textContent=Number.isFinite(elapsed)?(elapsed<60?elapsed+' sec':Math.floor(elapsed/60)+' min '+elapsed%60+' sec'):'';
}
function renderState(s){
  const previous=appState;appState=s;selectedIDs=new Set([...selectedIDs].filter(id=>s.documents.some(d=>d.id===id)));if(selectedID&&!s.documents.some(d=>d.id===selectedID))selectedID=[...selectedIDs][0]||'';if(previous&&currentTab!=='settings'&&(previous.settings.ip!==s.settings.ip||previous.settings.port!==s.settings.port)){ $('settingIP').value=s.settings.ip;$('settingPort').value=s.settings.port;}if(!initialized){populateSettings(s.settings);initialized=true;if(s.documents.length)selectDocument(s.documents[0].id);}
  $('version').textContent=s.version;$('docCount').textContent=s.documents.length;
  $('outputShort').textContent=s.settings.output.replace(/^\/Users\/[^/]+\//,'').replaceAll('/',' / ');$('outputShort').title=s.settings.output;
  renderDevices(s);
  const c=s.connection,online=c.status==='online';
  $('connectionText').textContent=online?'Last check successful':c.status==='error'?'Check connection':'Not checked';$('connectionBadge').title=c.checked?'Checked: '+formatDate(c.checked):'Scanner status has not been read';
  ['sideDot','headerDot'].forEach(id=>{$(id).classList.toggle('online',online);$(id).classList.toggle('error',c.status==='error');});
  const j=s.job;native('busy',{busy:j.busy});
  $('taskMessage').textContent=j.message;$('taskIndicator').className='task-indicator'+(j.busy?' running':j.stage==='done'?' done':j.stage==='error'?' error':'');
  $('viewDiagnostics').classList.toggle('hidden',!j.started);
  $('startScan').classList.toggle('hidden',j.busy&&j.kind==='scan');$('stopScan').classList.toggle('hidden',!(j.busy&&j.kind==='scan'));
  ['startScan','headerCheck','settingsCheck','saveSettings','chooseFolder','paper','dpi','mode','format','prefix','settingIP','settingPort','settingOutput'].forEach(id=>{$(id).disabled=j.busy||requestBusy;});
  document.querySelectorAll('[data-source]').forEach(b=>b.disabled=j.busy||requestBusy);updateOptions();
  $('diagScannerState').textContent=c.scannerState==='Idle'?'Idle':c.scannerState||'Not read';
  $('diagChecked').textContent=c.checked?'Last check: '+formatDate(c.checked):'Click Check Connection to read status';
  $('diagADF').textContent=c.paperInADF==='true'?'Paper detected':c.paperInADF==='false'?'No paper detected':'Not read';
  $('diagJob').textContent=j.busy?'Running':j.stage==='done'?'Completed':j.stage==='error'?'Incomplete':j.stage==='stopped'?'Stopped':'No active job';$('diagPages').textContent='Received '+j.pages+' pages';
  const logs=j.logs.join('\n');if(logs!==lastLog){lastLog=logs;const nearBottom=$('logText').scrollHeight-$('logText').scrollTop-$('logText').clientHeight<55;$('logText').textContent=logs||'No logs yet. Connection checks and scan activity appear here.';if(nearBottom)$('logText').scrollTop=$('logText').scrollHeight;}
  $('openLog').disabled=j.busy||!j.dir;
  if(j.ended&&j.ended!==lastCompleted){lastCompleted=j.ended;if(j.documentID)selectDocument(j.documentID);}
  renderFiles();renderPreview();updateElapsed();
}
async function poll(){try{const s=await api('state');unavailableCount=0;renderState(s);}catch(e){if(++unavailableCount===3){toast('Cannot reach the local scan service. Reopen the app; logs will be kept.',true);}}finally{setTimeout(poll,appState?.job?.busy?550:1700);}}
async function save(quiet=false){const ss=settingsFromUI();if(currentTab!=='settings'&&appState){ss.ip=appState.settings.ip;ss.port=appState.settings.port;}await api('settings',ss);const s=await api('state');renderState(s);if(!quiet)toast('Settings saved.');}
async function operate(kind){
  if(requestBusy||appState?.job?.busy)return;requestBusy=true;
  try{await save(true);native('busy',{busy:true});await api(kind,{});renderState(await api('state'));}catch(e){toast(e.message,true);}finally{requestBusy=false;if(appState)renderState(appState);}
}
async function openFile(action){try{await api('open',{action,id:selectedID});}catch(e){toast(e.message,true);}}

function renderDevices(s){
  const discovery=s.discovery||{},devices=discovery.devices||[];
  const chosen=devices.find(d=>d.ip===s.settings.ip&&d.port===s.settings.port);
  const deviceName=chosen?.name||'Scanner';
  $('deviceName').textContent=deviceName;$('deviceName').title=deviceName;
  $('deviceToggle').title=deviceName+' · '+s.settings.ip+':'+s.settings.port;
  $('sideIP').title=s.settings.ip+':'+s.settings.port;
  $('deviceSelect').title=deviceName+' · '+s.settings.ip+':'+s.settings.port;
  $('sideStatus').textContent=discovery.busy?'Searching…':s.connection.status==='online'?'Connected':devices.length?'No supported scanner connected':'No devices found';
  $('sideIP').textContent=s.settings.ip;
  $('discoveryMessage').textContent=discovery.busy?'Searching the local network…':discovery.message||'Use Refresh to search for scanners.';
  const options=JSON.stringify([devices,s.settings.ip,s.settings.port]);
  if($('deviceSelect').dataset.signature!==options){
    $('deviceSelect').dataset.signature=options;$('deviceSelect').replaceChildren();
    const placeholder=document.createElement('option');placeholder.textContent=devices.length?'Choose a scanner':'No devices found';placeholder.value='';$('deviceSelect').append(placeholder);
    for(const d of devices){const option=document.createElement('option');option.value=JSON.stringify([d.ip,d.port]);option.textContent=d.name+' · '+d.ip+(d.supported?'':' — scanning not supported');option.disabled=!d.supported;option.selected=d.ip===s.settings.ip&&d.port===s.settings.port&&d.supported;$('deviceSelect').append(option);}
  }
  $('deviceSelect').disabled=Boolean(discovery.busy||s.job.busy);
  $('discoverDevices').disabled=Boolean(discovery.busy||s.job.busy);
}
function beginRename(){
  const doc=currentDocument();if((currentTab==='files'&&selectedDocuments().length!==1)||!doc||editingTitle||appState?.job?.busy||fileActionBusy||trashTarget)return;
  editingTitle={id:doc.id,title:doc.title};$('titleEdit').value=doc.title;$('titleEdit').classList.remove('hidden');$('previewTitle').classList.add('hidden');$('titleEdit').focus();$('titleEdit').select();
}
async function finishRename(cancel=false){
  if(!editingTitle)return;const edit=editingTitle,value=$('titleEdit').value;editingTitle=null;
  $('titleEdit').classList.add('hidden');$('previewTitle').classList.remove('hidden');
  if(cancel||value===edit.title){renderPreview();if(cancel)focusDocument(false);return;}
  try{const doc=await api('rename',{id:edit.id,title:value});renderState(await api('state'));toast('Renamed to “'+doc.title+'”.');}
  catch(e){toast('Rename failed: '+e.message,true);renderPreview();}
}
function focusDocument(scroll=true){
  const row=[...$('documentList').children].find(el=>el.dataset.id===selectedID);
  if(row){row.focus({preventScroll:true});if(scroll)row.scrollIntoView({block:'nearest'});}
}
function editableTarget(target){return Boolean(target?.closest('input,textarea,select,[contenteditable="true"]'));}
function closeFileMenu(){ $('fileMenu').classList.add('hidden'); }
function showFileMenu(x,y){
  if(appState?.job?.busy||fileActionBusy||trashTarget)return;
  const menu=$('fileMenu');menu.classList.remove('hidden');
  menu.style.left=Math.max(8,Math.min(x,innerWidth-menu.offsetWidth-8))+'px';
  menu.style.top=Math.max(8,Math.min(y,innerHeight-menu.offsetHeight-8))+'px';
  const count=selectedDocuments().length;
  menu.querySelector('[data-file-action="rename"]').disabled=count!==1;
  menu.querySelector('[data-file-action="copy"]').firstChild.textContent=count>1?'Copy '+count+' Documents ':currentDocument()?.pdf?'Copy PDF ':'Copy Images ';
  menu.querySelector('[data-file-action="trash"]').firstChild.textContent=count>1?'Move '+count+' to Trash… ':'Move to Trash… ';
  menu.querySelector('button:not(:disabled)').focus();
}
async function copyDocument(){
  const docs=selectedDocuments();if(!docs.length||appState?.job?.busy||fileActionBusy||trashTarget)return;
  fileActionBusy=true;renderFiles();
  try{await api('copy-files',selectionPayload(docs));toast(docs.length>1?docs.length+' documents copied. Paste them in Finder or another app.':docs[0].pdf?'PDF copied. Paste it in Finder or another app.':'Original images copied. Paste them in Finder or another app.');}
  catch(e){toast('Copy failed: '+e.message,true);}
  finally{fileActionBusy=false;renderFiles();}
}
window.studioCopySelection = () => {
  if(currentTab!=='files'||editableTarget(document.activeElement)||trashTarget||String(window.getSelection()).length||!currentDocument())return false;
  closeFileMenu();copyDocument();return true;
};
function showTrashTargets(){
  $('trashHeading').textContent=trashTarget.length>1?'Move '+trashTarget.length+' documents to Trash?':'Move document to Trash?';
  $('trashName').textContent=trashTarget.map(d=>d.title).join('\n');
}
function askTrash(){
  const docs=selectedDocuments();if(!docs.length||appState?.job?.busy||fileActionBusy||trashTarget)return;
  closeFileMenu();trashTarget=docs.map(d=>({id:d.id,title:d.title}));showTrashTargets();$('trashError').textContent='';
  $('trashDialog').classList.remove('hidden');$('cancelTrash').focus();
}
function cancelTrash(){
  if(fileActionBusy)return;trashTarget=null;$('trashDialog').classList.add('hidden');focusDocument(false);
}
async function confirmTrash(){
  if(!trashTarget||fileActionBusy)return;
  const targets=trashTarget.slice(),ids=new Set(targets.map(d=>d.id)),docs=visibleDocuments();
  const idx=docs.findIndex(d=>d.id===selectedID),remaining=docs.filter(d=>!ids.has(d.id));
  const next=docs.slice(idx+1).find(d=>!ids.has(d.id))||remaining[remaining.length-1];fileActionBusy=true;
  $('confirmTrash').disabled=true;$('cancelTrash').disabled=true;renderFiles();
  try{
    const result=await api('trash',{...selectionPayload(targets),confirmed:true});
    const moved=new Set(result.moved||targets.map(d=>d.id));
    appState.documents=appState.documents.filter(d=>!moved.has(d.id));
    const failed=targets.filter(d=>!moved.has(d.id));
    if(failed.length){
      // Retry can only target the remaining items, never already moved documents.
      trashTarget=failed;selectedIDs=new Set(failed.map(d=>d.id));previewDocument(failed[0].id);showTrashTargets();
      $('trashError').textContent=moved.size+' moved to Trash; '+failed.length+' could not be moved. '+Object.values(result.failed||{}).join(' ');
    }else{
      trashTarget=null;$('trashDialog').classList.add('hidden');selectDocument(next?.id||'');
      toast(targets.length>1?'Moved '+targets.length+' documents to Trash.':'Moved “'+targets[0].title+'” to Trash.');
    }
    try{renderState(await api('state'));}catch(e){toast('File operation finished. Refresh history to update the list.',true);}
    if(!trashTarget)focusDocument();
  }catch(e){if(trashTarget)$('trashError').textContent=e.message;}
  finally{fileActionBusy=false;$('confirmTrash').disabled=false;$('cancelTrash').disabled=false;renderFiles();}
}
function fileKeys(e){
  if(trashTarget){
    if(e.key==='Escape'){e.preventDefault();cancelTrash();}
    if(e.key==='Tab'){e.preventDefault();if(!fileActionBusy)(document.activeElement===$('cancelTrash')?$('confirmTrash'):$('cancelTrash')).focus();}
    return;
  }
  if(!$('fileMenu').classList.contains('hidden')){
    const buttons=[...$('fileMenu').querySelectorAll('button:not(:disabled)')];
    if(e.key==='Escape'||e.key==='Tab'){e.preventDefault();closeFileMenu();focusDocument(false);}
    else if(['ArrowDown','ArrowUp'].includes(e.key)){e.preventDefault();const idx=buttons.indexOf(document.activeElement);buttons[(idx+(e.key==='ArrowDown'?1:buttons.length-1))%buttons.length].focus();}
    return;
  }
  if(e.defaultPrevented||currentTab!=='files'||editableTarget(e.target)||e.target.closest('nav')||e.isComposing||e.metaKey||e.ctrlKey||e.altKey)return;
  if(['ArrowUp','ArrowDown'].includes(e.key)){
    e.preventDefault();const docs=visibleDocuments();if(!docs.length)return;
    const idx=docs.findIndex(d=>d.id===selectedID),next=idx<0?0:Math.max(0,Math.min(docs.length-1,idx+(e.key==='ArrowDown'?1:-1)));
    chooseDocument(docs[next].id,e);focusDocument();
  }else if((e.key==='Enter'&&e.target.closest('.document-item'))||e.key==='F2'){e.preventDefault();beginRename();}
  else if(e.key==='Delete'||e.key==='Backspace'){e.preventDefault();askTrash();}
  else if(e.key==='ContextMenu'){e.preventDefault();const row=[...$('documentList').children].find(el=>el.dataset.id===selectedID);if(row){const b=row.getBoundingClientRect();showFileMenu(b.left,b.top);}}
}
function bind(){
  $('renameFile').addEventListener('click',beginRename);
  $('copyFile').addEventListener('click',copyDocument);
  $('trashFile').addEventListener('click',askTrash);
  $('cancelTrash').addEventListener('click',cancelTrash);
  $('confirmTrash').addEventListener('click',confirmTrash);
  document.querySelectorAll('[data-file-action]').forEach(button=>button.addEventListener('click',()=>{
    closeFileMenu();const action=button.dataset.fileAction;
    if(action==='rename')beginRename();else if(action==='copy'){focusDocument(false);copyDocument();}else askTrash();
  }));
  document.addEventListener('pointerdown',e=>{if(!e.target.closest('#fileMenu'))closeFileMenu();});
  window.addEventListener('resize',closeFileMenu);
  document.addEventListener('keydown',fileKeys);
  // Native Edit > Copy and Command-C both dispatch the WebView copy event.
  // Preserve ordinary copying whenever the user is editing or selecting text.
  document.addEventListener('copy',e=>{
    if(currentTab!=='files'||editableTarget(e.target)||editableTarget(document.activeElement)||trashTarget||String(window.getSelection()).length||!currentDocument())return;
    e.preventDefault();closeFileMenu();copyDocument();
  });
  $('deviceToggle').addEventListener('click',()=>{const open=$('deviceMenu').classList.contains('hidden');$('deviceMenu').classList.toggle('hidden',!open);$('deviceToggle').setAttribute('aria-expanded',String(open));});
  document.addEventListener('click',e=>{if(!e.target.closest('.device-picker')){$('deviceMenu').classList.add('hidden');$('deviceToggle').setAttribute('aria-expanded','false');}});
  $('manualConnection').addEventListener('click',()=>{switchTab('settings');$('deviceMenu').classList.add('hidden');$('settingIP').focus();});
  $('discoverDevices').addEventListener('click',async()=>{try{await api('discover',{});renderState(await api('state'));}catch(e){toast(e.message,true);}});
  $('deviceSelect').addEventListener('change',async()=>{if(!$('deviceSelect').value)return;const [ip,port]=JSON.parse($('deviceSelect').value);try{await api('select-device',{ip,port});const s=await api('state');populateSettings(s.settings);renderState(s);toast('Scanner selected.');}catch(e){toast(e.message,true);}});
  $('previewTitle').addEventListener('click',beginRename);
  $('previewTitle').addEventListener('keydown',e=>{if(e.key==='Enter'||e.key===' '){e.preventDefault();beginRename();}});
  $('titleEdit').addEventListener('keydown',e=>{if(e.isComposing)return;if(e.key==='Enter'||e.key==='Escape'){e.preventDefault();e.stopPropagation();finishRename(e.key==='Escape');}});
  $('titleEdit').addEventListener('blur',()=>finishRename());

  document.querySelectorAll('[data-tab]').forEach(b=>b.addEventListener('click',()=>switchTab(b.dataset.tab)));
  document.querySelectorAll('[data-source]').forEach(b=>b.addEventListener('click',()=>{setSource(b.dataset.source);updateOptions();}));
  $('dpi').addEventListener('change',updateOptions);$('mode').addEventListener('change',updateOptions);$('format').addEventListener('change',updateOptions);
  $('startScan').addEventListener('click',()=>operate('scan'));$('headerCheck').addEventListener('click',()=>operate('check'));$('settingsCheck').addEventListener('click',()=>operate('check'));
  $('stopScan').addEventListener('click',async()=>{try{await api('cancel',{});toast('Stop requested. Press Cancel on the printer if needed.');}catch(e){toast(e.message,true);}});
  $('saveSettings').addEventListener('click',async()=>{try{await save();}catch(e){toast(e.message,true);}});
  $('changeOutput').addEventListener('click',()=>{switchTab('settings');$('settingOutput').focus();});
  $('chooseFolder').addEventListener('click',()=>{if(!native('chooseFolder',{path:$('settingOutput').value}))toast('The system folder picker is unavailable in a test browser. Enter an absolute path instead.',true);});
  $('fileSearch').addEventListener('input',()=>{
    const docs=visibleDocuments();selectedIDs=new Set(docs.filter(d=>selectedIDs.has(d.id)).map(d=>d.id));
    selectionAnchor='';if(!selectedIDs.size)selectDocument(docs[0]?.id||'');else previewDocument(selectedIDs.has(selectedID)?selectedID:[...selectedIDs][0]);
  });
  $('refreshFiles').addEventListener('click',async()=>{try{await api('refresh',{});renderState(await api('state'));toast('Scan history refreshed.');}catch(e){toast(e.message,true);}});
  $('prevPage').addEventListener('click',()=>{pageIndex--;rotation=0;renderPreview();});$('nextPage').addEventListener('click',()=>{pageIndex++;rotation=0;renderPreview();});
  $('previewImage').addEventListener('load',applyZoom);$('previewImage').addEventListener('error',()=>{if(currentDocument())toast('Cannot read this image. It may have been moved. Refresh scan history.',true);});
  $('zoom').addEventListener('change',()=>{previewSignature='';renderPreview();});$('rotatePreview').addEventListener('click',()=>{rotation=(rotation+90)%360;renderPreview();toast('Only the preview is rotated; saved images and PDFs are unchanged.');});
  $('openPDF').addEventListener('click',()=>openFile('pdf'));$('revealFile').addEventListener('click',()=>openFile('reveal'));$('openOutput').addEventListener('click',()=>openFile('output'));$('openLog').addEventListener('click',()=>openFile('diagnostics'));
  $('errorDetails').addEventListener('click',()=>switchTab('diagnostics'));$('viewDiagnostics').addEventListener('click',()=>switchTab('diagnostics'));
  $('openPrivacy').addEventListener('click',()=>{if(!native('privacy'))toast('System Settings > Privacy & Security > Local Network.');});
  $('copyLog').addEventListener('click',async()=>{const text=appState?.job?.logs?.join('\n')||'No logs yet';if(native('copy',{text})){toast('Log copied.');return;}try{await navigator.clipboard.writeText(text);toast('Log copied.');}catch(e){toast('Select the log and press Command-C.');}});
  document.addEventListener('keydown',e=>{if(trashTarget||e.defaultPrevented)return;if(e.metaKey&&['1','2','3','4'].includes(e.key)){e.preventDefault();switchTab(['scan','files','settings','diagnostics'][Number(e.key)-1]);}if(e.metaKey&&e.key==='Enter'){e.preventDefault();operate('scan');}});
  // Tab buttons support vertical keyboard navigation.
  document.querySelector('nav').addEventListener('keydown',e=>{if(!['ArrowDown','ArrowUp'].includes(e.key))return;e.preventDefault();const all=[...document.querySelectorAll('[data-tab]')],idx=all.indexOf(document.activeElement),next=all[(idx+(e.key==='ArrowDown'?1:3))%4];next.focus();switchTab(next.dataset.tab);});
  $('scanPreviewMount').appendChild($('previewPanel'));renderPreview();setInterval(updateElapsed,1000);poll();
}
bind();
