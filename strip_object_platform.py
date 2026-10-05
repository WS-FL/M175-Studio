# Swift's downstream Linux lld artificially rejects Mach-O object platform records.
# Remove only that record from our own .o files; the final link explicitly sets
# the real macOS minimum (12.0) and SDK (15.0) on the executable.
import struct,sys
from pathlib import Path
for fn in sys.argv[1:]:
    p=Path(fn);b=bytearray(p.read_bytes());assert struct.unpack_from('<I',b)[0]==0xfeedfacf
    n,size=struct.unpack_from('<II',b,16);pos=32;keep=[]
    for i in range(n):
        cmd,sz=struct.unpack_from('<II',b,pos)
        if cmd not in [0x32,0x24,0x25,0x2f,0x30]:keep.append(b[pos:pos+sz])
        pos+=sz
    cmds=b''.join(keep);b[32:32+size]=cmds+b'\x00'*(size-len(cmds));struct.pack_into('<II',b,16,len(keep),len(cmds));p.write_bytes(b)
