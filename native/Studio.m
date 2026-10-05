// M175 Studio — native AppKit + WKWebView host. Manual reference counting.
// The scan engine is a child of this GUI executable, not of Terminal or osascript.
#if __has_include(<Cocoa/Cocoa.h>) && !defined(M175_CROSS_BUILD)
#import <Cocoa/Cocoa.h>
#import <WebKit/WebKit.h>
#else
#import "MacABI.h"
#endif

@interface StudioDelegate : NSObject <NSApplicationDelegate, NSWindowDelegate, WKScriptMessageHandler, WKNavigationDelegate> {
    NSWindow *window;
    WKWebView *web;
    NSTask *engine;
    NSPipe *lifetimePipe;
    NSFileHandle *logHandle;
    NSTimer *readyTimer;
    NSString *readyPath;
    NSString *sessionDir;
    NSString *serviceURL;
    BOOL busy;
    BOOL failed;
    NSInteger ticks;
}
@end

static NSMenuItem *MenuItem(NSString *title,SEL action,NSString *key,id target){
    NSMenuItem *item=[[[NSMenuItem alloc] initWithTitle:title action:action keyEquivalent:key] autorelease];
    if(target)[item setTarget:target];return item;
}
static NSString *JSString(NSString *s){
    NSData *data=[NSJSONSerialization dataWithJSONObject:[NSArray arrayWithObjects:s,nil] options:0 error:NULL];
    NSString *json=[[[NSString alloc] initWithData:data encoding:4] autorelease];
    return [NSString stringWithFormat:@"(%@)[0]",json];
}
@implementation StudioDelegate
- (void)showAbout:(id)sender{[self showSettings:sender];}
- (void)showSettings:(id)sender{[web evaluateJavaScript:@"window.studioShowSettings && window.studioShowSettings()" completionHandler:nil];[window makeKeyAndOrderFront:nil];}
- (void)installMenu{
    NSMenu *bar=[[[NSMenu alloc] initWithTitle:@""] autorelease];
    NSMenu *appMenu=[[[NSMenu alloc] initWithTitle:@"M175 Studio"] autorelease];
    [appMenu addItem:MenuItem(@"About M175 Studio",@selector(showAbout:),@"",self)];
    [appMenu addItem:[NSMenuItem separatorItem]];
    [appMenu addItem:MenuItem(@"Settings...",@selector(showSettings:),@",",self)];
    [appMenu addItem:[NSMenuItem separatorItem]];
    [appMenu addItem:MenuItem(@"Quit M175 Studio",@selector(terminate:),@"q",[NSApplication sharedApplication])];
    NSMenuItem *appRoot=MenuItem(@"M175 Studio",(SEL)0,@"",nil);[appRoot setSubmenu:appMenu];[bar addItem:appRoot];
    NSMenu *edit=[[[NSMenu alloc] initWithTitle:@"Edit"] autorelease];
    [edit addItem:MenuItem(@"Undo",@selector(undo:),@"z",nil)];
    [edit addItem:[NSMenuItem separatorItem]];
    [edit addItem:MenuItem(@"Cut",@selector(cut:),@"x",nil)];
    [edit addItem:MenuItem(@"Copy",@selector(copy:),@"c",nil)];
    [edit addItem:MenuItem(@"Paste",@selector(paste:),@"v",nil)];
    [edit addItem:MenuItem(@"Select All",@selector(selectAll:),@"a",nil)];
    NSMenuItem *editRoot=MenuItem(@"Edit",(SEL)0,@"",nil);[editRoot setSubmenu:edit];[bar addItem:editRoot];
    NSMenu *win=[[[NSMenu alloc] initWithTitle:@"Window"] autorelease];
    [win addItem:MenuItem(@"Minimize",@selector(performMiniaturize:),@"m",nil)];
    [win addItem:MenuItem(@"Close Window",@selector(performClose:),@"w",nil)];
    NSMenuItem *winRoot=MenuItem(@"Window",(SEL)0,@"",nil);[winRoot setSubmenu:win];[bar addItem:winRoot];
    [[NSApplication sharedApplication] setMainMenu:bar];
}
- (void)showStartupError:(NSString*)message{
    if(failed)return;failed=YES;if(readyTimer){[readyTimer invalidate];readyTimer=nil;}
    // The dynamic message is passed as JSON text, not interpolated as HTML.
    [web loadHTMLString:@"<!doctype html><meta charset='utf-8'><style>body{font:14px/1.8 -apple-system,sans-serif;background:#f6f8f2;color:#465b3d;margin:70px;max-width:760px}h1{font-size:26px;font-weight:600}pre{white-space:pre-wrap;word-break:break-word;background:#edf1e8;padding:20px;border-radius:10px;color:#89947d;font-size:12px}</style><h1>M175 Studio could not start</h1><p>The scan engine did not become ready. Quit the app, move the entire M175 Studio.app into Applications and open it again.</p><p>Keep the app bundle intact. Detailed errors are recorded in:</p><pre>~/Library/Logs/M175 Studio/engine.log</pre><p>If macOS cannot verify the developer, review the exact message in System Settings > Privacy & Security.</p>" baseURL:nil];
}
- (void)applicationDidFinishLaunching:(NSNotification*)note{
    [self installMenu];
    NSRect frame=NSMakeRect(0,0,1120,790);
    window=[[NSWindow alloc] initWithContentRect:frame styleMask:(1|2|4|8) backing:2 defer:NO];
    [window setTitle:@"M175 Studio"];
    [window setTitlebarAppearsTransparent:YES];
    [window setMinSize:NSMakeSize(960,690)];
    [window setReleasedWhenClosed:NO];
    [window setDelegate:self];
    if(![window setFrameUsingName:@"M175StudioMainWindow"])[window center];
    [window setFrameAutosaveName:@"M175StudioMainWindow"];
    WKWebViewConfiguration *config=[[[WKWebViewConfiguration alloc] init] autorelease];
    [[config userContentController] addScriptMessageHandler:self name:@"studio"];
    web=[[WKWebView alloc] initWithFrame:frame configuration:config];
    [web setAutoresizingMask:(2|16)];[web setNavigationDelegate:self];[web setAllowsBackForwardNavigationGestures:NO];[web setAllowsLinkPreview:NO];
    [window setContentView:web];
    [web loadHTMLString:@"<!doctype html><meta charset='utf-8'><style>body{margin:0;height:100vh;display:grid;place-items:center;background:#f7f8f5;font:13px -apple-system,sans-serif;color:#78906d;text-align:center}h1{font-size:25px;color:#356848;font-weight:600}p{margin-top:18px}</style><div><h1>M175 Studio</h1><p>Starting the scan workspace...</p></div>" baseURL:nil];
    [window makeKeyAndOrderFront:nil];[[NSApplication sharedApplication] activateIgnoringOtherApps:YES];
    NSFileManager *fm=[NSFileManager defaultManager];
    NSDictionary *privateDir=[NSDictionary dictionaryWithObject:[NSNumber numberWithUnsignedLong:0700] forKey:NSFilePosixPermissions];
    // A test-only profile keeps runtime checks away from user settings and scans.
    NSString *testRoot=[[[NSProcessInfo processInfo] environment] objectForKey:@"M175_STUDIO_TEST_ROOT"];
    if(testRoot && ![testRoot isAbsolutePath])testRoot=nil;
    NSString *cache=testRoot?[testRoot stringByAppendingPathComponent:@"cache"]:[NSHomeDirectory() stringByAppendingPathComponent:@"Library/Caches/M175 Studio"];
    sessionDir=[[cache stringByAppendingPathComponent:[[NSUUID UUID] UUIDString]] retain];
    NSError *error=nil;
    if(![fm createDirectoryAtPath:sessionDir withIntermediateDirectories:YES attributes:privateDir error:&error]){[self showStartupError:[error localizedDescription]];return;}
    readyPath=[[sessionDir stringByAppendingPathComponent:@"ready.json"] retain];
    NSString *logDir=testRoot?[testRoot stringByAppendingPathComponent:@"logs"]:[NSHomeDirectory() stringByAppendingPathComponent:@"Library/Logs/M175 Studio"];
    [fm createDirectoryAtPath:logDir withIntermediateDirectories:YES attributes:privateDir error:NULL];
    NSString *logPath=[logDir stringByAppendingPathComponent:@"engine.log"];
    if(![fm fileExistsAtPath:logPath])[fm createFileAtPath:logPath contents:nil attributes:[NSDictionary dictionaryWithObject:[NSNumber numberWithUnsignedLong:0600] forKey:NSFilePosixPermissions]];
    logHandle=[[NSFileHandle fileHandleForWritingAtPath:logPath] retain];[logHandle seekToEndOfFile];
#if defined(__arm64__) || defined(__aarch64__)
    NSString *binary=@"m175-engine-arm64";
#else
    NSString *binary=@"m175-engine-x86_64";
#endif
    NSString *enginePath=[[[NSBundle mainBundle] resourcePath] stringByAppendingPathComponent:binary];
    engine=[[NSTask alloc] init];[engine setExecutableURL:[NSURL fileURLWithPath:enginePath]];
    NSMutableArray *arguments=[NSMutableArray arrayWithObjects:@"--ready-file",readyPath,@"--watch-stdin",nil];
    if(testRoot){[arguments addObject:@"--support-dir"];[arguments addObject:[testRoot stringByAppendingPathComponent:@"support"]];}
    [engine setArguments:arguments];
    lifetimePipe=[[NSPipe pipe] retain];[engine setStandardInput:lifetimePipe];
    if(logHandle){[engine setStandardOutput:logHandle];[engine setStandardError:logHandle];}
    if(![engine launchAndReturnError:&error]){[self showStartupError:[error localizedDescription]];return;}
    readyTimer=[NSTimer scheduledTimerWithTimeInterval:0.1 target:self selector:@selector(checkReady:) userInfo:nil repeats:YES];
}
- (void)checkReady:(NSTimer*)timer{
    ticks++;
    NSData *data=[NSData dataWithContentsOfFile:readyPath];
    if(data){
        NSDictionary *obj=[NSJSONSerialization JSONObjectWithData:data options:0 error:NULL];
        NSString *url=[obj isKindOfClass:[NSDictionary class]]?[obj objectForKey:@"url"]:nil;
        if([url isKindOfClass:[NSString class]]&&[url hasPrefix:@"http://127.0.0.1:"]){
            serviceURL=[url retain];[timer invalidate];readyTimer=nil;
            [web loadRequest:[NSURLRequest requestWithURL:[NSURL URLWithString:serviceURL]]];return;
        }
    }
    if(ticks>200||![engine isRunning]){[self showStartupError:@"The scan engine could not start. Check the logs."];}
}
- (void)userContentController:(WKUserContentController*)controller didReceiveScriptMessage:(WKScriptMessage*)message{
    if(![[message frameInfo] isMainFrame]||!serviceURL||![[[web URL] absoluteString] hasPrefix:serviceURL])return;
    id body=[message body];if(![body isKindOfClass:[NSDictionary class]])return;
    NSString *action=[body objectForKey:@"action"];if(![action isKindOfClass:[NSString class]])return;
    if([action isEqualToString:@"busy"]){id b=[body objectForKey:@"busy"];if([b isKindOfClass:[NSNumber class]])busy=[b boolValue];return;}
    if([action isEqualToString:@"copy"]){
        NSString *text=[body objectForKey:@"text"];if([text isKindOfClass:[NSString class]]&&[text length]<1048576){NSPasteboard *pb=[NSPasteboard generalPasteboard];[pb clearContents];[pb setString:text forType:NSPasteboardTypeString];}return;
    }
    if([action isEqualToString:@"privacy"]){
        NSURL *u=[NSURL URLWithString:@"x-apple.systempreferences:com.apple.preference.security?Privacy_LocalNetwork"];
        if(![[NSWorkspace sharedWorkspace] openURL:u])[[NSWorkspace sharedWorkspace] openURL:[NSURL fileURLWithPath:@"/System/Applications/System Settings.app"]];return;
    }
    if([action isEqualToString:@"chooseFolder"]){
        NSOpenPanel *panel=[NSOpenPanel openPanel];[panel setCanChooseFiles:NO];[panel setCanChooseDirectories:YES];[panel setCanCreateDirectories:YES];[panel setAllowsMultipleSelection:NO];[panel setTitle:@"Choose where to save scans"];[panel setPrompt:@"Choose Folder"];
        NSString *path=[body objectForKey:@"path"];if([path isKindOfClass:[NSString class]]&&[path length]>0)[panel setDirectoryURL:[NSURL fileURLWithPath:[path stringByExpandingTildeInPath]]];
        [panel beginSheetModalForWindow:window completionHandler:^(NSInteger result){
            if(result==1){NSString *p=[[panel URL] path];if(p)[web evaluateJavaScript:[NSString stringWithFormat:@"window.studioFolderSelected(%@)",JSString(p)] completionHandler:nil];}
        }];return;
    }
}
- (void)webView:(WKWebView*)view decidePolicyForNavigationAction:(WKNavigationAction*)action decisionHandler:(void(^)(WKNavigationActionPolicy))handler{
    NSString *url=[[[action request] URL] absoluteString];
    // No remote page can acquire access to the native bridge or the scanner.
    BOOL allow=[url isEqualToString:@"about:blank"]||(serviceURL!=nil&&[url hasPrefix:serviceURL]);
    handler(allow?1:0);
}
- (void)webView:(WKWebView*)view didFailProvisionalNavigation:(id)navigation withError:(NSError*)error{if([error code]==-999)return;[self showStartupError:[error localizedDescription]];}
- (BOOL)windowShouldClose:(id)sender{
    if(busy){[web evaluateJavaScript:@"window.studioQuitBusy && window.studioQuitBusy()" completionHandler:nil];return NO;}return YES;
}
- (NSApplicationTerminateReply)applicationShouldTerminate:(NSApplication*)sender{
    if(busy){[web evaluateJavaScript:@"window.studioQuitBusy && window.studioQuitBusy()" completionHandler:nil];return 0;}return 1;
}
- (BOOL)applicationShouldTerminateAfterLastWindowClosed:(NSApplication*)sender{return YES;}
- (BOOL)applicationSupportsSecureRestorableState:(NSApplication*)sender{return YES;}
- (void)applicationWillTerminate:(NSNotification*)note{
    if(readyTimer)[readyTimer invalidate];
    [[lifetimePipe fileHandleForWriting] closeFile];
    if([engine isRunning])[engine terminate];
    [logHandle closeFile];
    if(sessionDir)[[NSFileManager defaultManager] removeItemAtPath:sessionDir error:NULL];
}
@end
int main(int argc,const char *argv[]){
    @autoreleasepool {
        NSApplication *app=[NSApplication sharedApplication];[app setActivationPolicy:0];
        StudioDelegate *delegate=[[StudioDelegate alloc] init];[app setDelegate:delegate];[app run];
        [delegate release];
    }
    return 0;
}
