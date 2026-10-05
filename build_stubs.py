from pathlib import Path
root=Path(__file__).parent/'build'/'link-stubs';root.mkdir(parents=True,exist_ok=True)
def st(name,install,syms):
    txt='--- !tapi-tbd\ntbd-version: 4\ntargets: [ arm64-macos, x86_64-macos ]\ninstall-name: '+repr(install)+'\ncurrent-version: 1.0\ncompatibility-version: 1.0\nexports:\n  - targets: [ arm64-macos, x86_64-macos ]\n    symbols: [ '+', '.join(repr(s) for s in syms)+' ]\n...\n'
    (root/(name+'.tbd')).write_text(txt)
def cls(names):return ['_OBJC_CLASS_$_'+n for n in names.split()]
st('AppKit','/System/Library/Frameworks/AppKit.framework/Versions/C/AppKit',cls('NSApplication NSWindow NSMenu NSMenuItem NSOpenPanel NSPasteboard NSWorkspace')+['_NSPasteboardTypeString'])
st('Foundation','/System/Library/Frameworks/Foundation.framework/Versions/C/Foundation',cls('NSArray NSBundle NSData NSDictionary NSFileHandle NSFileManager NSJSONSerialization NSNumber NSObject NSPipe NSString NSTask NSTimer NSURL NSURLRequest NSUUID')+['_OBJC_METACLASS_$_NSObject','_NSFilePosixPermissions','_NSHomeDirectory'])
st('WebKit','/System/Library/Frameworks/WebKit.framework/Versions/A/WebKit',cls('WKWebView WKWebViewConfiguration'))
st('CoreFoundation','/System/Library/Frameworks/CoreFoundation.framework/Versions/A/CoreFoundation',['___CFConstantStringClassReference'])
st('libobjc','/usr/lib/libobjc.A.dylib',['__objc_empty_cache','_objc_alloc','_objc_alloc_init','_objc_autorelease','_objc_autoreleasePoolPop','_objc_autoreleasePoolPush','_objc_msgSend','_objc_release','_objc_retain'])
st('libSystem','/usr/lib/libSystem.B.dylib',['__Block_object_assign','__Block_object_dispose','__NSConcreteStackBlock','dyld_stub_binder','___stack_chk_guard','___stack_chk_fail'])
