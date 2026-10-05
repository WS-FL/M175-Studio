#!/usr/bin/env python3
"""Ad-hoc Mach-O signing for these self-built binaries only; not Developer ID.
Adds a deterministic LC_UUID when absent. Includes Info.plist/resources in
special slots for the GUI executable. Verifies every generated SHA-256 slot.
No entitlements, sandbox exceptions, hardened-runtime exceptions, or identity
certificates are added. This does not bypass Gatekeeper or notarize an app.
"""
from pathlib import Path
import hashlib,struct,sys,uuid
H=lambda b:hashlib.sha256(b).digest()
def align(n,m):return (n+m-1)//m*m
def commands(b):
    assert struct.unpack_from('<I',b)[0]==0xfeedfacf
    n,size=struct.unpack_from('<II',b,16);p=32
    for _ in range(n):
        c,s=struct.unpack_from('<II',b,p);yield p,c,s;p+=s
    assert p==32+size

def resign(path,identifier,info=None,resources=None):
    path=Path(path);b=bytearray(path.read_bytes());cmds=list(commands(b))
    oldsig=next((p for p,c,s in cmds if c==0x1d),None)
    code_end=struct.unpack_from('<I',b,oldsig+8)[0] if oldsig is not None else align(len(b),16)
    b=b[:code_end]+bytearray(max(0,code_end-len(b)))
    n,size=struct.unpack_from('<II',b,16);new=[]
    if not any(c==0x1b for p,c,s in cmds):
        digest=bytearray(H(b+identifier.encode())[:16]);digest[6]=(digest[6]&15)|0x50;digest[8]=(digest[8]&63)|0x80
        new.append(struct.pack('<II',0x1b,24)+digest)
    if oldsig is None:new.append(struct.pack('<IIII',0x1d,16,0,0))
    extra=b''.join(new)
    # Header padding only; do not move sections or alter relocation targets.
    end=32+size
    if extra:
        assert not any(b[end:end+len(extra)]),'No free Mach-O header padding'
        b[end:end+len(extra)]=extra;struct.pack_into('<II',b,16,n+len(new),size+len(extra))
    cmdlist=list(commands(b));sig=next(p for p,c,s in cmdlist if c==0x1d)
    segments={b[p+8:p+24].rstrip(b'\x00').decode():p for p,c,s in cmdlist if c==0x19}
    text=segments['__TEXT'];textoff,textsize=struct.unpack_from('<QQ',b,text+40)
    link=segments['__LINKEDIT'];linkoff=struct.unpack_from('<Q',b,link+40)[0]
    ident=identifier.encode()+b'\x00';special={}
    # Empty internal requirements are valid for an ad-hoc signature.
    req=struct.pack('>III',0xfade0c01,12,0) if info is not None else None
    if info is not None:special[1]=H(Path(info).read_bytes());special[2]=H(req)
    if resources is not None:special[3]=H(Path(resources).read_bytes())
    ns=max(special,default=0);slots=(code_end+4095)//4096
    hashOffset=88+len(ident)+ns*32;cdsize=hashOffset+slots*32
    count=2 if req else 1;sbhead=12+8*count;total=sbhead+cdsize+(len(req) if req else 0)
    struct.pack_into('<IIII',b,sig,0x1d,16,code_end,total)
    struct.pack_into('<Q',b,link+48,code_end+total-linkoff)
    struct.pack_into('<Q',b,link+32,align(code_end+total-linkoff,16384))
    # CodeDirectory 0x20400, SHA-256, 4 KiB pages, ADHOC flag only.
    cd=struct.pack('>9I4B4I4Q',0xfade0c02,cdsize,0x20400,0x2,hashOffset,88,ns,slots,code_end,32,2,0,12,0,0,0,0,0,textoff,textsize,1)
    assert len(cd)==88
    cd+=ident+b''.join(special.get(i,b'\x00'*32) for i in range(ns,0,-1))
    cd+=b''.join(H(b[i:min(i+4096,code_end)]) for i in range(0,code_end,4096))
    sb=struct.pack('>III',0xfade0cc0,total,count)+struct.pack('>II',0,sbhead)
    if req:sb+=struct.pack('>II',2,sbhead+len(cd))
    out=b+sb+cd+(req or b'');assert len(out)==code_end+total
    path.write_bytes(out);path.chmod(0o755);verify(path)
    return H(cd)[:20]

def verify(path):
    b=Path(path).read_bytes();sig=next(p for p,c,s in commands(b) if c==0x1d)
    off,size=struct.unpack_from('<II',b,sig+8);magic,ln,count=struct.unpack_from('>III',b,off);assert magic==0xfade0cc0 and ln==size
    dirs={struct.unpack_from('>I',b,off+12+i*8)[0]:struct.unpack_from('>I',b,off+16+i*8)[0] for i in range(count)}
    cd=off+dirs[0];hdr=struct.unpack_from('>9I4B4I4Q',b,cd)
    _,length,version,flags,hashOff,identOff,ns,nc,limit,hashSize,hashType,platform,pageBits,*_=hdr
    assert limit==off and hashSize==32 and hashType==2
    for i in range(nc):assert b[cd+hashOff+i*32:cd+hashOff+(i+1)*32]==H(b[i*4096:min((i+1)*4096,limit)]),f'Hash mismatch {i}'
    return {'identifier':b[cd+identOff:].split(b'\x00',1)[0].decode(),'pages':nc,'specialSlots':ns,'uuid':next(b[p+8:p+24].hex() for p,c,s in commands(b) if c==0x1b),'cdhash':H(b[cd:cd+length])[:20].hex()}

def fat(paths,destination):
    parts=[Path(p).read_bytes() for p in paths];hdr=struct.pack('>II',0xcafebabe,len(parts));entries=[];pos=align(8+20*len(parts),16384)
    for b in parts:
        cpu,sub=struct.unpack_from('<II',b,4);entries.append((pos,b));hdr+=struct.pack('>5I',cpu,sub,pos,len(b),14);pos=align(pos+len(b),16384)
    out=bytearray(hdr)
    for pos,b in entries:out+=b'\x00'*(pos-len(out));out+=b
    Path(destination).write_bytes(out);Path(destination).chmod(0o755)
if __name__=='__main__':
    if sys.argv[1]=='verify':print(verify(sys.argv[2]))
    else:resign(sys.argv[1],sys.argv[2],*(sys.argv[3:]))
