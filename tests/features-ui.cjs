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
 browser=await chromium.launch({headless:true});const page=await browser.newPage({viewport:{width:1120,height:790}}),errors=[];
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
 // Component fixture: the real HTTP discovery/protocol probes are covered in Go.
 let discovery={busy:true,devices:[],message:'Searching…'},selected=null;
 await page.route('**/api/state',async route=>{const res=await route.fetch(),s=await res.json();s.discovery=discovery;if(selected){s.settings.ip=selected.ip;s.settings.port=selected.port;s.connection={status:'online',ip:selected.ip};}await route.fulfill({json:s});});
 await page.route('**/api/select-device',async route=>{selected=route.request().postDataJSON();await route.fulfill({json:{ok:true}});});
 await page.waitForFunction(()=>document.getElementById('sideIP').textContent==='Searching…');
 await page.locator('#deviceToggle').click();assert(await page.locator('#discoverDevices').isDisabled());
 discovery={busy:false,devices:[],message:'No scanners found.'};await page.waitForFunction(()=>document.getElementById('discoveryMessage').textContent==='No scanners found.');
 discovery={busy:false,devices:[{ip:'192.168.5.44',port:8289,name:'HP M175nw',supported:true},{ip:'192.168.5.45',port:8289,name:'Second M175nw',supported:true},{ip:'192.168.5.46',port:8289,name:'Other printer',supported:false}],message:'Connected'};
 await page.waitForFunction(()=>document.querySelectorAll('#deviceSelect option').length===4);
 assert(await page.locator('#deviceSelect option').last().isDisabled());
 await page.locator('#deviceSelect').selectOption(JSON.stringify(['192.168.5.45',8289]));await page.waitForFunction(()=>document.getElementById('sideIP').textContent.includes('192.168.5.45'));assert.equal(selected.ip,'192.168.5.45');
 await page.screenshot({path:path.join(shot,'03-device-picker.png')});
 await page.setViewportSize({width:960,height:665});assert(await page.evaluate(()=>document.documentElement.scrollWidth<=960));await page.screenshot({path:path.join(shot,'04-small-window.png')});
 assert.deepEqual(errors,[]);
 const report={tests:['English Enter save','Chinese blur save','Escape cancel','illegal characters','empty title','duplicate title suffix','poll retains editor','IME Enter ignored','PDF and JPEG references','file list and restart history','discovery searching/none/multiple/unsupported UI','device selection','960px layout'],javascriptErrors:errors};fs.writeFileSync(path.join(shot,'report.json'),JSON.stringify(report,null,2));console.log(JSON.stringify(report,null,2));
})().catch(e=>{console.error(e);process.exitCode=1}).finally(async()=>{if(browser)await browser.close();proc.kill();});
