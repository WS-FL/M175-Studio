#!/usr/bin/env python3
"""Real macOS GUI regression: Cmd-C -> independent reader -> Finder Cmd-V.

Run on an unlocked Mac after quitting M175 Studio. Requires Accessibility access
for the terminal. Uses an isolated app profile and disposable files; replaces the
clipboard and opens only test Finder folders. Never sends messages to other apps.
Build the app first. Artifacts are retained under ignored build/ for inspection.
"""
import base64
import json
import os
from pathlib import Path
import re
import subprocess
import tempfile
import time
import urllib.request

REPO = Path(__file__).resolve().parents[1]
APP = REPO / 'mac-build/M175 Studio.app'

def script(source):
    return subprocess.check_output(['osascript', '-e', source], text=True).strip()

def quote(value):
    return json.dumps(str(value))

def keys(source):
    script('tell application "System Events" to tell process "M175Studio"\nset frontmost to true\n' + source + '\nend tell')

assert script('tell application "System Events" to exists process "M175Studio"') == 'false', 'Quit M175 Studio before running this test'
assert script('tell application "System Events" to get UI elements enabled') == 'true', 'Accessibility access required'
root = Path(tempfile.mkdtemp(prefix='native-clipboard-', dir=REPO / 'build'))
(root / 'support').mkdir()
jpeg = base64.b64decode(re.search(r"Buffer.from\('([^']+)'", (REPO / 'tests/features-ui.cjs').read_text())[1])
for i in (1, 2):
    folder = root / 'scans' / f'scan-test-{i}'
    folder.mkdir(parents=True)
    (folder / 'page-001.jpg').write_bytes(jpeg)
    # Finder copies file contents without needing to parse a PDF.
    (folder / f'剪贴板 Test {i}.pdf').write_bytes(b'%PDF-1.4\n% Clipboard regression fixture\n%%EOF\n')
    (folder / 'm175-studio.json').write_text(json.dumps(dict(title=f'Clipboard Test {i}', created=f'2026-01-01T12:00:0{i}Z', dpi=300, source='platen', paper='Letter', mode='color')))
(root / 'support/settings.json').write_text(json.dumps(dict(ip='192.0.2.1', port=8289, output=str(root / 'scans'), source='platen', paper='Letter', dpi=300, mode='color', format='pdf', prefix='Test')))
reader = root / 'read-clipboard'
subprocess.run(['xcrun', 'clang', '-framework', 'Cocoa', '-fno-objc-arc', str(REPO / 'tests/ClipboardReadback.m'), '-o', str(reader)], check=True)
process = subprocess.Popen([str(APP / 'Contents/MacOS/M175Studio')], env={**os.environ, 'M175_STUDIO_TEST_ROOT': str(root)}, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
try:
    for _ in range(100):
        ready = list((root / 'cache').glob('*/ready.json'))
        if ready: break
        assert process.poll() is None, 'Test app exited'
        time.sleep(.1)
    assert ready, 'No ready file'
    url = json.loads(ready[0].read_text())['url']
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))
    state = json.load(opener.open(url + 'api/state'))
    assert len(state['documents']) == 2 and not state['job']['busy']
    time.sleep(2)
    keys('keystroke "2" using command down')
    time.sleep(.5)
    results = []
    for count in (1, 2):
        keys(('key code 125 using shift down\n' if count == 2 else '') + 'keystroke "c" using command down')
        time.sleep(.7)
        paths = [Path(p) for p in json.loads(subprocess.check_output([str(reader)]))]
        assert len(paths) == count, paths
        assert all(p.is_relative_to(root / 'scans') for p in paths), 'Unexpected clipboard source'
        dest = root / f'finder-paste-{count}'
        dest.mkdir()
        script('tell application "Finder"\nactivate\nopen POSIX file '+quote(dest)+'\nend tell\ndelay 0.7\ntell application "System Events" to tell process "Finder" to keystroke "v" using command down')
        for _ in range(40):
            if all((dest / p.name).exists() for p in paths): break
            time.sleep(.1)
        assert len(list(dest.iterdir())) == count
        assert all((dest / p.name).read_bytes() == p.read_bytes() for p in paths)
        results.append(f'{count} file(s): native GUI copy, independent clipboard read, Finder paste, filename and byte equality passed')
    print(json.dumps({'version': state['version'], 'results': results, 'thirdPartyChatPaste': 'not tested'}, indent=2))
finally:
    if process.poll() is None:
        keys('keystroke "q" using command down')
        process.wait(timeout=10)
