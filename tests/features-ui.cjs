// Run after building build/m175-engine. Requires Node + Playwright Chromium.
const {chromium}=require('playwright');
const fs=require('fs'),path=require('path'),os=require('os'),cp=require('child_process'),assert=require('assert');
const root=path.resolve(__dirname,'..'),temp=fs.mkdtempSync(path.join(os.tmpdir(),'m175-features-'));
const support=path.join(temp,'support'),output=path.join(temp,'scans');fs.mkdirSync(support);fs.mkdirSync(output);
// A tiny valid JPEG; the app must retain it byte for byte throughout renames.
const jpeg=Buffer.from('/9j/4AAQSkZJRgABAQAAAQABAAD/2wBDAP//////////////////////////////////////////////////////////////////////////////////////2wBDAf//////////////////////////////////////////////////////////////////////////////////////wAARCAABAAEDASIAAhEBAxEB/8QAFQABAQAAAAAAAAAAAAAAAAAAAAf/xAAUEAEAAAAAAAAAAAAAAAAAAAAA/8QAFQEBAQAAAAAAAAAAAAAAAAAAAAX/xAAUEQEAAAAAAAAAAAAAAAAAAAAA/9oADAMBAAIRAxEAPwCwAB//2Q==','base64');
for(const [id,title] of [['scan-one','Scan · 2026-10-19 09.01.02'],['scan-two','Second scan']]){
 const dir=path.join(output,id);fs.mkdirSync(dir);fs.writeFileSync(path.join(dir,'page-001.jpg'),jpeg);fs.writeFileSync(path.join(dir,'original.pdf'),'%PDF-1.4\nfixture');
 fs.writeFileSync(path.join(dir,'m175-studio.json'),JSON.stringify({title,created:'2026-10-19T09:01:02-05:00',dpi:300,source:'platen',paper:'Letter',mode:'color'}));
}
fs.writeFileSync(path.join(support,'settings.json'),JSON.stringify({ip:'192.168.5.44',port:8289,output,source:'platen',paper:'Letter',dpi:300,mode:'color',format:'pdf',prefix:'Scan'}));
const ready=path.join(temp,'ready.json');const proc=cp.spawn(path.join(root,'build/m175-engine'),['--support-dir',support,'--ready-file',ready,'--no-discovery']);
let stderr='';proc.stderr.on('data',b=>stderr+=b);let browser;
(async()=>{
 for(let i=0;i<100&&!fs.existsSync(ready);i++)await new Promise(r=>setTimeout(r,100));
 assert(fs.existsSync(ready),stderr);const url=JSON.parse(fs.readFileSync(ready)).url;
 browser=await chromium.launch({headless:true,executablePath:process.env.M175_TEST_BROWSER||undefined});const page=await browser.newPage({viewport:{width:1120,height:790}}),errors=[];
 page.on('pageerror',e=>errors.push(e.message));await page.goto(url);await page.waitForFunction(()=>document.getElementById('previewTitle').textContent.includes('Scan'));
 const title=page.locator('#previewTitle'),input=page.locator('#titleEdit');
 async function rename(text,key='Enter'){
  const before=await title.innerText();await title.click();await input.fill(text);
  if(key==='blur')await page.locator('#previewMeta').click();else await input.press(key);
  await page.waitForFunction(()=>document.getElementById('titleEdit').classList.contains('hidden'));
  if(key!=='Escape')await page.waitForFunction(before=>document.getElementById('previewTitle').textContent!==before,before);
 }
 await rename('English title');assert.equal(await title.innerText(),'English title');
 await rename('中文 扫描 结果','blur');assert.equal(await title.innerText(),'中文 扫描 结果');
 await rename('cancelled change','Escape');assert.equal(await title.innerText(),'中文 扫描 结果');
 await rename('a/b:c*?<>|"');assert.equal(await title.innerText(),'a b c');
 await rename('   ');assert.equal(await title.innerText(),'Untitled scan');
 await rename('Second scan');assert.equal(await title.innerText(),'Second scan 2');
 // Polling must not replace in-progress input, and IME Enter must not save.
 await title.click();await input.fill('编辑中');await page.waitForTimeout(1900);assert.equal(await input.inputValue(),'编辑中');
 await input.dispatchEvent('keydown',{key:'Enter',isComposing:true,bubbles:true});assert(await input.isVisible());await input.press('Escape');
 const state=await (await fetch(url+'api/state')).json();const doc=state.documents.find(d=>d.id==='scan-one');assert.equal(doc.pdf,'Second scan 2.pdf');assert(fs.readFileSync(path.join(output,doc.id,'page-001.jpg')).equals(jpeg));
 await page.locator('#tab-files').click();assert((await page.locator('.document-item strong').allTextContents()).includes('Second scan 2'));
 await page.reload();await page.waitForFunction(()=>document.getElementById('docCount').textContent==='2');assert.equal(await title.innerText(),'Second scan 2');
 const shot=path.join(root,'build/feature-ui');fs.mkdirSync(shot,{recursive:true});await title.hover();await page.screenshot({path:path.join(shot,'01-title-hover.png')});
 await title.click();await input.fill('研究记录');await page.screenshot({path:path.join(shot,'02-title-edit.png')});await input.press('Escape');
 // Real settings API must accept a campus address, persist it, and pass it
 // to Check Connection. Mock only the check to avoid reaching a real printer.
 await page.locator('#tab-settings').click();
 await page.locator('#settingIP').fill('198.51.100.6');
 let checkedIP=null;
 await page.route('**/api/check',async route=>{
  checkedIP=(await (await fetch(url+'api/state')).json()).settings.ip;
  await route.fulfill({json:{ok:true}});
 });
 const checkResponse=page.waitForResponse('**/api/check');
 await page.locator('#settingsCheck').click();
 await checkResponse;
 assert.equal(checkedIP,'198.51.100.6');
 assert.equal(JSON.parse(fs.readFileSync(path.join(support,'settings.json'))).ip,'198.51.100.6');
 await page.reload();await page.locator('#tab-settings').click();
 assert.equal(await page.locator('#settingIP').inputValue(),'198.51.100.6');
 await page.screenshot({path:path.join(shot,'06-campus-address-settings.png')});
 await page.locator('#tab-scan').click();
 // Component fixture: the real HTTP discovery/protocol probes are covered in Go.
 let discovery={busy:true,devices:[],message:'Searching…'},selected=null;
 await page.route('**/api/state',async route=>{const res=await route.fetch(),s=await res.json();s.discovery=discovery;if(selected){s.settings.ip=selected.ip;s.settings.port=selected.port;s.connection={status:'online',ip:selected.ip};}await route.fulfill({json:s});});
 await page.route('**/api/select-device',async route=>{selected=route.request().postDataJSON();await route.fulfill({json:{ok:true}});});
 await page.waitForFunction(()=>document.getElementById('sideStatus').textContent==='Searching…');
 await page.locator('#deviceToggle').click();assert(await page.locator('#discoverDevices').isDisabled());
 discovery={busy:false,devices:[],message:'No scanners found.'};await page.waitForFunction(()=>document.getElementById('discoveryMessage').textContent==='No scanners found.');
 discovery={busy:false,devices:[{ip:'192.168.5.44',port:8289,name:'HP M175nw',supported:true},{ip:'192.168.5.45',port:8289,name:'Second M175nw',supported:true},{ip:'192.168.5.46',port:8289,name:'Other printer',supported:false}],message:'Connected'};
 await page.waitForFunction(()=>document.querySelectorAll('#deviceSelect option').length===4);
 assert(await page.locator('#deviceSelect option').last().isDisabled());
 await page.locator('#deviceSelect').selectOption(JSON.stringify(['192.168.5.45',8289]));await page.waitForFunction(()=>document.getElementById('sideIP').textContent.includes('192.168.5.45'));assert.equal(selected.ip,'192.168.5.45');
 await page.screenshot({path:path.join(shot,'03-device-picker.png')});
 await page.setViewportSize({width:960,height:665});assert(await page.evaluate(()=>document.documentElement.scrollWidth<=960));await page.screenshot({path:path.join(shot,'04-small-window.png')});

 // Reproduce the reported sidebar overflow with real-length Bonjour names.
 for(const name of ['HP LaserJet 100 colorMFP M175nw (ABC123)', '办公室 打印扫描一体机 · 中文设备名称很长也应正确显示', 'HP '+ 'Long-device-name-'.repeat(16)]){
  discovery.devices[1].name=name;
  await page.waitForFunction(name=>document.getElementById('deviceName').textContent===name,name);
  assert.equal(await page.locator('#deviceName').getAttribute('title'),name);
  assert((await page.locator('#deviceToggle').getAttribute('title')).includes(name));
  for(const width of [1120,960]){
   await page.setViewportSize({width,height:665});
   const bounds=await page.evaluate(()=>{
    const sidebar=document.querySelector('.sidebar').getBoundingClientRect();
    const card=document.getElementById('deviceToggle').getBoundingClientRect();
    const name=document.getElementById('deviceName'),n=name.getBoundingClientRect();
    const arrow=document.querySelector('#deviceToggle>span:last-child').getBoundingClientRect();
    return {sidebarRight:sidebar.right,cardRight:card.right,nameRight:n.right,arrowLeft:arrow.left,arrowRight:arrow.right,clipped:name.scrollWidth>name.clientWidth,display:getComputedStyle(name).display,overflow:getComputedStyle(name).textOverflow,verticallyClipped:name.scrollHeight>name.clientHeight,scrollable:getComputedStyle(name).overflowY,cardBottom:card.bottom,cardTop:card.top,navBottom:document.querySelector('nav').getBoundingClientRect().bottom,footerBottom:document.querySelector('.build').getBoundingClientRect().bottom};
   });
   assert(bounds.cardRight<=bounds.sidebarRight,JSON.stringify(bounds));
   assert(bounds.nameRight<=bounds.arrowLeft,JSON.stringify(bounds));
   assert(bounds.arrowRight<=bounds.cardRight,JSON.stringify(bounds));
   assert.equal(bounds.display,'block');assert.equal(bounds.overflow,'clip');assert(!bounds.clipped);
   assert(bounds.navBottom<=bounds.cardTop);assert(bounds.footerBottom<=665);
   if(name.length<80)assert(!bounds.verticallyClipped);else assert.equal(bounds.scrollable,'auto');
  }
 }
 discovery.devices[1].name='HP LaserJet 100 colorMFP M175nw (ABC123)';
 for(const address of ['192.168.5.85','198.168.255.254','198.51.100.6','2001:db8:1234:5678:1234:5678:abcd:1234']){
  selected.ip=address;discovery.devices[1].ip=address;
  await page.waitForFunction(address=>document.getElementById('sideIP').textContent===address,address);
  for(const width of [1120,960]){
   await page.setViewportSize({width,height:665});
   const b=await page.locator('#sideIP').evaluate(el=>({width:el.clientWidth,scrollWidth:el.scrollWidth,height:el.clientHeight,scrollHeight:el.scrollHeight,overflow:getComputedStyle(el).textOverflow}));
   assert.equal(b.overflow,'clip');assert(b.scrollWidth<=b.width);assert(b.scrollHeight<=b.height);
   assert.equal(await page.locator('#sideStatus').innerText(),'Connected');
  }
 }
 selected.ip='192.168.5.85';discovery.devices[1].ip=selected.ip;
 await page.waitForFunction(()=>document.getElementById('sideIP').textContent==='192.168.5.85');
 await page.locator('#deviceToggle').click();
 await page.screenshot({path:path.join(shot,'05-long-name-sidebar.png')});
 assert.deepEqual(errors,[]);
 const report={tests:['campus IP Save and Check plus settings persistence/reload','English Enter save','Chinese blur save','Escape cancel','illegal characters','empty title','duplicate title suffix','poll retains editor','IME Enter ignored','PDF and JPEG references','file list and restart history','discovery searching/none/multiple/unsupported UI','device selection','960px layout','long English/Chinese Bonjour names at 1120px and 960px','wrapped full device name and contained dropdown arrow','full IPv4 and wrapped IPv6 addresses at minimum window size'],javascriptErrors:errors};fs.writeFileSync(path.join(shot,'report.json'),JSON.stringify(report,null,2));console.log(JSON.stringify(report,null,2));
})().catch(e=>{console.error(e);process.exitCode=1}).finally(async()=>{if(browser)await browser.close();proc.kill();});
