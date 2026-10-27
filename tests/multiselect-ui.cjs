// Run after building build/m175-engine. Requires Node + Playwright Chromium.
const {chromium}=require('playwright');
const fs=require('fs'),path=require('path'),os=require('os'),cp=require('child_process'),assert=require('assert');
const root=path.resolve(__dirname,'..'),temp=fs.mkdtempSync(path.join(os.tmpdir(),'m175-multiselect-'));
const support=path.join(temp,'support'),output=path.join(temp,'scans');fs.mkdirSync(support);fs.mkdirSync(output);
// A tiny valid JPEG; the app must retain it byte for byte throughout renames.
const jpeg=Buffer.from('/9j/4AAQSkZJRgABAQAAAQABAAD/2wBDAP//////////////////////////////////////////////////////////////////////////////////////2wBDAf//////////////////////////////////////////////////////////////////////////////////////wAARCAABAAEDASIAAhEBAxEB/8QAFQABAQAAAAAAAAAAAAAAAAAAAAf/xAAUEAEAAAAAAAAAAAAAAAAAAAAA/8QAFQEBAQAAAAAAAAAAAAAAAAAAAAX/xAAUEQEAAAAAAAAAAAAAAAAAAAAA/9oADAMBAAIRAxEAPwCwAB//2Q==','base64');
for(let i=1;i<=5;i++){
 const dir=path.join(output,'scan-'+i);fs.mkdirSync(dir);fs.writeFileSync(path.join(dir,'page-001.jpg'),jpeg);fs.writeFileSync(path.join(dir,'original.pdf'),'%PDF-1.4\nfixture');
 fs.writeFileSync(path.join(dir,'m175-studio.json'),JSON.stringify({title:'Document '+i,created:`2026-10-19T09:01:0${i}-05:00`,dpi:300,source:'platen',paper:'Letter',mode:'color'}));
}
fs.writeFileSync(path.join(support,'settings.json'),JSON.stringify({ip:'192.168.5.44',port:8289,output,source:'platen',paper:'Letter',dpi:300,mode:'color',format:'pdf',prefix:'Scan'}));
const ready=path.join(temp,'ready.json'),proc=cp.spawn(path.join(root,'build/m175-engine'),['--support-dir',support,'--ready-file',ready,'--no-discovery']);
let browser;let stderr='';proc.stderr.on('data',b=>stderr+=b);
(async()=>{
 for(let i=0;i<100&&!fs.existsSync(ready);i++)await new Promise(r=>setTimeout(r,100));assert(fs.existsSync(ready),stderr);
 const url=JSON.parse(fs.readFileSync(ready)).url;
 browser=await chromium.launch({headless:true,executablePath:process.env.M175_TEST_BROWSER||undefined});
 const page=await browser.newPage({viewport:{width:1120,height:790}}),errors=[];page.on('pageerror',e=>errors.push(e.message));
 let copies=[],trash=[],failID='';
 await page.route('**/api/copy-files',async route=>{copies.push(route.request().postDataJSON());await route.fulfill({json:{ok:true}});});
 await page.route('**/api/trash',async route=>{
  const body=route.request().postDataJSON();trash.push(body);assert.equal(body.confirmed,true);
  const moved=[],failed={};
  for(const id of body.ids||[body.id]){if(id===failID){failed[id]='Test permission denied';continue;}fs.renameSync(path.join(output,id),path.join(temp,'trashed-'+id));moved.push(id);}
  await fetch(url+'api/refresh',{method:'POST',headers:{'X-M175-Request':'1'},body:'{}'});
  await route.fulfill({json:{moved,failed}});
 });
 await page.goto(url);await page.waitForFunction(()=>document.getElementById('docCount').textContent==='5');await page.locator('#tab-files').click();
 const rows=page.locator('.document-item'),active=page.locator('.document-item[aria-pressed="true"]');
 const ids=await rows.evaluateAll(rows=>rows.map(r=>r.dataset.id));
 async function selection(want){assert.deepEqual(await active.evaluateAll(rows=>rows.map(r=>r.dataset.id)),want);assert.equal(await page.locator('#selectionCount').innerText(),want.length?want.length+' selected':'');}
 await rows.nth(0).click();await rows.nth(2).click({modifiers:['Meta']});await selection([ids[0],ids[2]]);assert(await page.locator('#renameFile').isDisabled());
 await rows.nth(2).click({modifiers:['Meta']});await selection([ids[0]]);
 await rows.nth(0).click({modifiers:['Meta']});await selection([]);assert(await page.locator('#copyFile').isDisabled());
 await rows.nth(1).click();await rows.nth(4).click({modifiers:['Shift']});await selection(ids.slice(1,5));
 await rows.nth(3).click({modifiers:['Shift']});await selection(ids.slice(1,4));
 await rows.nth(0).click({modifiers:['Shift']});await selection(ids.slice(0,2));
 await rows.nth(4).click({modifiers:['Meta']});await selection([ids[0],ids[1],ids[4]]);
 await page.waitForTimeout(1900);await selection([ids[0],ids[1],ids[4]]);
 await page.keyboard.press('Shift+ArrowUp');await selection(ids.slice(3,5));
 await page.keyboard.press('Shift+ArrowDown');await selection([ids[4]]);
 await rows.nth(0).click();await rows.nth(2).click({modifiers:['Shift']});
 await rows.nth(4).click({modifiers:['Meta','Shift']});await selection(ids);
 await page.keyboard.press('Meta+c');await page.waitForFunction(()=>document.getElementById('toast').textContent.startsWith('5 documents copied'));assert.deepEqual(copies.at(-1),{ids});
 assert(await page.evaluate(()=>window.studioCopySelection()));await page.waitForTimeout(150);assert.deepEqual(copies.at(-1),{ids});
 await rows.nth(2).click({button:'right'});await selection(ids);assert(await page.locator('[data-file-action="rename"]').isDisabled());
 await page.locator('[data-file-action="copy"]').click();await page.waitForTimeout(150);assert.deepEqual(copies.at(-1),{ids});
 await page.keyboard.press('Delete');assert.equal(await page.locator('#trashHeading').innerText(),'Move 5 documents to Trash?');
 assert.equal((await page.locator('#trashName').innerText()).split('\n').length,5);assert(await page.locator('#cancelTrash').evaluate(el=>el===document.activeElement));
 await page.keyboard.press('Enter');assert.equal(trash.length,0);await selection(ids);
 const shot=path.join(root,'build/multiselect-ui');fs.mkdirSync(shot,{recursive:true});await page.screenshot({path:path.join(shot,'01-multiple-selected.png')});
 await rows.nth(0).click();await rows.nth(1).click({modifiers:['Meta']});await rows.nth(3).click({button:'right'});await selection([ids[3]]);await page.keyboard.press('Escape');
 await rows.nth(0).click();await rows.nth(4).click({modifiers:['Shift']});
 await page.locator('#fileSearch').fill('Document 3');await selection([ids.find(id=>id==='scan-3')]);
 await page.locator('#fileSearch').press('Meta+a');await page.keyboard.press('Meta+c');const count=copies.length;await page.waitForTimeout(150);assert.equal(copies.length,count);
 await page.locator('#fileSearch').fill('');await selection(['scan-3']);
 await rows.nth(0).click();await rows.nth(2).click({modifiers:['Shift']});failID=ids[1];
 await page.locator('#trashFile').click();await page.screenshot({path:path.join(shot,'02-batch-confirmation.png')});
 await page.locator('#confirmTrash').click();await page.waitForFunction(()=>document.getElementById('trashError').textContent.includes('2 moved to Trash; 1 could not be moved'));
 await page.waitForFunction(()=>document.getElementById('docCount').textContent==='3');await selection([failID]);assert.equal(await page.locator('#trashHeading').innerText(),'Move document to Trash?');
 assert(fs.existsSync(path.join(output,failID)));assert(fs.existsSync(path.join(temp,'trashed-'+ids[0],'original.pdf')));
 failID='';await page.locator('#confirmTrash').click();await page.waitForFunction(()=>document.getElementById('docCount').textContent==='2');
 assert.deepEqual(trash[1],{id:ids[1],confirmed:true});assert.equal(await rows.count(),2);
 await page.setViewportSize({width:960,height:665});await rows.first().click();await rows.last().click({modifiers:['Shift']});
 assert(await page.locator('.file-actions').evaluate(el=>el.scrollWidth<=el.clientWidth));assert(await page.locator('.document-list').evaluate(el=>el.clientHeight>100));
 await page.keyboard.press('Backspace');await page.keyboard.press('Escape');assert.equal(trash.length,2);
 await page.locator('#trashFile').click();await page.locator('#confirmTrash').click();await page.waitForFunction(()=>document.getElementById('docCount').textContent==='0');
 assert(await page.locator('#copyFile').isDisabled());assert.equal(await page.locator('#previewTitle').innerText(),'Page Preview');
 await page.reload();await page.waitForFunction(()=>document.getElementById('docCount').textContent==='0');assert.deepEqual(errors,[]);
 const report={tests:['Command toggle including empty selection','Shift forward/reverse range and shrinking range','Command+Shift additive range','Shift+arrows','polling retains multiple selection','batch Command-C/native menu/context copy','right-click preserves selected group or selects unselected target','batch confirmation count and exact titles','default cancel and Escape retain selection','search excludes hidden selections and preserves text copy','partial Trash results and retry only failed target','delete remaining batch and empty history reload','960px controls'],javascriptErrors:errors};fs.writeFileSync(path.join(shot,'report.json'),JSON.stringify(report,null,2));console.log(JSON.stringify(report,null,2));
})().catch(e=>{console.error(e);process.exitCode=1}).finally(async()=>{if(browser)await browser.close();proc.kill();});
