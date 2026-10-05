from pathlib import Path
from http.server import BaseHTTPRequestHandler,ThreadingHTTPServer
from xml.etree import ElementTree as ET
import subprocess,threading,tempfile,time,json,urllib.request,sys,io,struct,os,base64,urllib.error
from PIL import Image,ImageDraw,ImageFont
from playwright.sync_api import sync_playwright
ROOT=Path(__file__).resolve().parents[1]
OUT=ROOT/'build'/'ui-test';OUT.mkdir(exist_ok=True)
T=Path(tempfile.mkdtemp(prefix='m175-studio-test-'))
font='/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf'
canvas=Image.new('RGB',(2550,3300),'white');d=ImageDraw.Draw(canvas)
d.rectangle((160,180,2390,430),fill='#216B56')
d.text((220,250),'M175 STUDIO',font=ImageFont.truetype(font,78),fill='white')
d.text((190,590),'Wireless scan / end-to-end test',font=ImageFont.truetype(font,58),fill='#284839')
d.text((190,700),'SIMULATED PRINTER DATA — NOT A REAL SCAN',font=ImageFont.truetype(font,31),fill='#a0aaa0')
for y in range(940,1830,100): d.rounded_rectangle((200,y,2280,y+25),radius=8,fill='#dce5da')
for i,c in enumerate(['#286950','#72a37c','#d7dfb0','#ceaa76','#9aacc4']):d.rounded_rectangle((200+i*405,2210,550+i*405,2480),radius=20,fill=c)
d.text((190,2940),'PAGE 01  /  300 DPI  /  LETTER',font=ImageFont.truetype(font,36),fill='#698064')
buf=io.BytesIO();canvas.save(buf,format='JPEG',quality=91);jpeg=buf.getvalue()
def envelope(s):return ('<SOAP-ENV:Envelope xmlns:SOAP-ENV="http://www.w3.org/2003/05/soap-envelope"><SOAP-ENV:Body>'+s+'</SOAP-ENV:Body></SOAP-ENV:Envelope>').encode()
def record(flags,typ,b):
 t=typ.encode();pad=lambda v:v+b'\0'*((-len(v))%4)
 return struct.pack('>BBHHHI',8|flags,16,0,0,len(t),len(b))+pad(t)+pad(b)
def dime(j):return record(4,'http://www.w3.org/2003/05/soap-envelope',envelope('<RetrieveImageResponse/>'))+record(1,'image/jpeg',j[:len(j)//2])+record(2,'',j[len(j)//2:])
class Mock(BaseHTTPRequestHandler):
 pages=0;source='Platen';adf=True;calls=[]
 def log_message(self,*args):pass
 def do_POST(self):
  b=self.rfile.read(int(self.headers['Content-Length'])).decode();Mock.calls.append(b)
  ct='application/soap+xml; charset=utf-8';code=202
  if 'GetScannerElements' in b:
   raw=envelope('<ScanElements><ScannerConfiguration/><ScannerStatus><ScannerState>Idle</ScannerState><PaperInADF>'+('true' if Mock.adf else 'false')+'</PaperInADF></ScannerStatus></ScanElements>')
  elif 'CreateScanJobRequest' in b:
   Mock.pages=0;Mock.source='ADF' if '>ADF<' in b else 'Platen';raw=envelope('<CreateScanJobResponse><JobId>42</JobId><JobToken>test-token</JobToken></CreateScanJobResponse>')
  elif 'GetJobInfo' in b:raw=envelope('<JobSummaryType><JobState>'+('Completed' if Mock.pages>=2 else 'Pending')+'</JobState></JobSummaryType>')
  elif 'RetrieveImageRequest' in b:Mock.pages+=1;raw=dime(jpeg);code=200;ct='application/dime'
  else:code=400;raw=envelope('<Fault/>')
  self.send_response(code);self.send_header('Content-Type',ct);self.send_header('Content-Length',str(len(raw)));self.end_headers();self.wfile.write(raw)
mock=ThreadingHTTPServer(('127.0.0.1',0),Mock);threading.Thread(target=mock.serve_forever,daemon=True).start()
ready=T/'ready.json';proc=subprocess.Popen([str(ROOT/'build'/'m175-engine-linux'),'--ready-file',str(ready),'--support-dir',str(T/'support')],stdout=subprocess.PIPE,stderr=subprocess.PIPE)
try:
 for _ in range(100):
  if ready.exists():break
  time.sleep(.1)
 assert ready.exists(),proc.stderr.read().decode()
 url=json.loads(ready.read_text())['url']
 def req(path,data=None):
  r=urllib.request.Request(url+'api/'+path,data=json.dumps(data).encode() if data is not None else None,headers={'Content-Type':'application/json','X-M175-Request':'1'});return json.loads(urllib.request.urlopen(r).read())
 ss=req('state')['settings'];ss.update(ip='127.0.0.1',port=mock.server_port,output=str(T/'scans'));req('settings',ss)
 with sync_playwright() as p:
  browser=p.chromium.launch(executable_path='/usr/bin/chromium',headless=True,args=['--no-sandbox'])
  page=browser.new_page(viewport={'width':1120,'height':790},device_scale_factor=1)
  errors=[];page.on('pageerror',lambda e:errors.append(str(e)))
  # Chromium in this environment blocks every URL navigation by policy. Keep
  # that policy intact. Render our own components with set_content, and bridge
  # fetch to the already-running local mock-backed service through Playwright.
  # The native WKWebView network transport still requires a Mac runtime test.
  def bridge(path,options):
   path=path.removeprefix('api/')
   body=json.loads(options['body']) if options and 'body' in options else None
   try:
    data=req(path,body)
    if path=='state':
     data['__testAssets']={}
     for doc in data['documents']:
      for name in doc['pages']:
       raw=(T/'scans'/doc['id']/name).read_bytes()
       data['__testAssets'][doc['id']+'/'+name]='data:image/jpeg;base64,'+base64.b64encode(raw).decode()
    return {'status':200,'data':data}
   except urllib.error.HTTPError as e:return {'status':e.code,'data':json.loads(e.read())}
  page.expose_function('__testBackend',bridge)
  html=(ROOT/'engine/ui/index.html').read_text()
  css=(ROOT/'engine/ui/style.css').read_text()
  js=(ROOT/'engine/ui/app.js').read_text()
  js=js.replace("return 'asset/'+encodeURIComponent(doc.id)+'/'+encodeURIComponent(name);", "return (window.__testAssets||{})[doc.id+'/'+name]||'';")
  harness="window.fetch=async(path,options)=>{const r=await window.__testBackend(path,options||{});if(r.data.__testAssets)window.__testAssets=r.data.__testAssets;return {ok:r.status>=200&&r.status<300,json:async()=>r.data};};"
  html=html.replace('<link rel="stylesheet" href="style.css">','<style>'+css+'</style>').replace('<script src="app.js" defer></script>','').replace('</body>','<script>'+harness+js+'</script></body>')
  page.set_content(html);page.wait_for_function("document.getElementById('settingIP').value==='127.0.0.1'")
  page.screenshot(path=str(OUT/'01-scan-empty.png'))
  page.click('#headerCheck');page.wait_for_function("document.getElementById('taskMessage').textContent.includes('Connected. Scanner responded')",timeout=10000)
  page.click('#startScan');page.wait_for_function("document.getElementById('taskMessage').textContent.includes('Scan complete')",timeout=20000)
  page.wait_for_function("document.getElementById('previewImage').naturalWidth>0")
  page.screenshot(path=str(OUT/'02-scan-completed.png'))
  state=req('state');assert len(state['documents'])==1 and state['documents'][0]['pdf']
  pdf=T/'scans'/state['documents'][0]['id']/state['documents'][0]['pdf']
  import fitz
  doc=fitz.open(pdf);assert len(doc)==1 and abs(doc[0].rect.width-612)<.1;doc[0].get_pixmap(matrix=fitz.Matrix(.6,.6)).save(str(OUT/'pdf-render.png'))
  page.click('#tab-files');page.screenshot(path=str(OUT/'03-files.png'))
  page.fill('#fileSearch','not-found');assert page.locator('.empty-list').count()==1
  page.fill('#fileSearch','');assert page.locator('.document-item').count()==1
  page.click('#tab-settings');page.screenshot(path=str(OUT/'04-settings.png'))
  page.click('#tab-diagnostics');page.screenshot(path=str(OUT/'05-diagnostics.png'))
  assert 'HTTP 200; application/dime' in page.locator('#logText').inner_text()
  # Gray PDF keeps color original, via the actual GUI and backend.
  page.click('#tab-scan');page.select_option('#mode','gray');page.click('#startScan')
  page.wait_for_function("document.getElementById('docCount').textContent==='2'",timeout=20000)
  state=req('state');assert state['documents'][0]['mode']=='gray' and state['documents'][0]['pages'][0].startswith('gray-')
  # ADF two-page workflow under HTTP 202 and chunked DIME.
  page.select_option('#mode','color');page.click('[data-source="adf"]');page.click('#startScan')
  page.wait_for_function("document.getElementById('docCount').textContent==='3'",timeout=20000)
  state=req('state');assert len(state['documents'][0]['pages'])==2
  page.click('#nextPage');assert page.locator('#pageNumber').inner_text()=='2 / 2'
  # No-paper error stays inside the same window; no new tab or modal.
  Mock.adf=False;page.click('#startScan');page.wait_for_function("document.getElementById('previewError').classList.contains('hidden')===false",timeout=15000)
  page.screenshot(path=str(OUT/'06-inline-error.png'))
  assert 'ADF' in req('state')['job']['error'];assert len(browser.contexts[0].pages)==1
  assert not errors,errors
  # Minimum-size layout and controls remain reachable (scrolling permitted).
  page.set_viewport_size({'width':960,'height':665});page.click('#tab-settings');page.screenshot(path=str(OUT/'07-small-window.png'))
  assert page.evaluate('document.documentElement.scrollWidth')<=960
  report={'browser':'Chromium component rendering + local IPC test harness on Linux. Browser URL policy left intact. Not a native macOS WKWebView runtime test.','javascript_errors':errors,'tests':['4 tab navigation','HTTP 202 connection','flatbed scan','JPEG preview','PDF rendered at Letter size','history search','local grayscale PDF with originals','2-page ADF simulation','inline no-paper error','single-window flow','minimum window layout'],'output':str(OUT),'test_temp':str(T)}
  (OUT/'report.json').write_text(json.dumps(report,indent=2));print(json.dumps(report,indent=2));browser.close()
finally:
 mock.shutdown();proc.terminate()
 try:proc.wait(timeout=5)
 except subprocess.TimeoutExpired:proc.kill()
