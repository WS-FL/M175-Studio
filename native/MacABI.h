// Minimal, independently written public Cocoa ABI declarations for cross-compilation.
// These are declarations only: no Apple SDK binaries, headers, or private APIs are bundled.
// A normal Mac build uses Apple's installed public framework headers instead.
#ifndef M175_MAC_ABI
#define M175_MAC_ABI
#if defined(__arm64__) || defined(__aarch64__)
typedef _Bool BOOL;
#else
typedef signed char BOOL;
#endif
#define YES ((BOOL)1)
#define NO ((BOOL)0)
#define nil ((id)0)
#define NULL ((void*)0)
typedef unsigned long NSUInteger;
typedef long NSInteger;
typedef NSInteger WKNavigationActionPolicy;
typedef NSInteger NSApplicationTerminateReply;
typedef double NSTimeInterval;
typedef double CGFloat;
typedef struct {CGFloat x,y;} NSPoint;
typedef struct {CGFloat width,height;} NSSize;
typedef struct {NSPoint origin;NSSize size;} NSRect;
static inline NSRect NSMakeRect(CGFloat x,CGFloat y,CGFloat w,CGFloat h){NSRect r={{x,y},{w,h}};return r;}
static inline NSSize NSMakeSize(CGFloat w,CGFloat h){NSSize r={w,h};return r;}
@class NSString, NSArray, NSDictionary, NSData, NSError, NSURL, NSNotification, NSApplication, NSWindow, NSView, NSFileHandle, NSPipe, NSMenu, NSMenuItem, WKWebView;
__attribute__((objc_root_class)) @interface NSObject { Class isa; }
+ (id)alloc;+ (id)new;+ (Class)class;- (id)init;- (id)autorelease;- (id)retain;- (oneway void)release;- (BOOL)isKindOfClass:(Class)c;
@end
@interface NSString : NSObject
+ (id)stringWithFormat:(NSString*)fmt,...;
- (NSString*)stringByAppendingPathComponent:(NSString*)s;
- (NSString*)stringByAppendingString:(NSString*)s;
- (NSString*)stringByExpandingTildeInPath;
- (BOOL)isEqualToString:(NSString*)s;
- (BOOL)hasPrefix:(NSString*)s;
- (BOOL)isAbsolutePath;
- (NSUInteger)length;
- (const char*)UTF8String;
- (id)initWithData:(NSData*)d encoding:(NSUInteger)e;
@end
@interface NSArray : NSObject
+ (id)arrayWithObjects:(id)first,...;
@end
@interface NSMutableArray : NSArray
- (void)addObject:(id)object;
@end
@interface NSProcessInfo : NSObject
+ (id)processInfo;
- (NSDictionary*)environment;
@end
@interface NSDictionary : NSObject
+ (id)dictionaryWithObject:(id)v forKey:(id)k;
- (id)objectForKey:(id)k;
@end
@interface NSNumber : NSObject
+ (id)numberWithUnsignedLong:(unsigned long)n;
- (BOOL)boolValue;
@end
@interface NSData : NSObject
+ (id)dataWithContentsOfFile:(NSString*)p;
@end
@interface NSJSONSerialization : NSObject
+ (id)JSONObjectWithData:(NSData*)d options:(NSUInteger)o error:(NSError**)e;
+ (NSData*)dataWithJSONObject:(id)o options:(NSUInteger)n error:(NSError**)e;
@end
@interface NSUUID : NSObject
+ (id)UUID;- (NSString*)UUIDString;
@end
@interface NSURL : NSObject
+ (id)fileURLWithPath:(NSString*)s;
+ (id)URLWithString:(NSString*)s;
- (NSString*)path;- (NSString*)absoluteString;
@end
@interface NSURLRequest : NSObject
+ (id)requestWithURL:(NSURL*)u;
@end
@interface NSError : NSObject
- (NSString*)localizedDescription;- (NSInteger)code;
@end
@interface NSBundle : NSObject
+ (id)mainBundle;- (NSString*)resourcePath;
- (NSURL*)executableURL;
@end
@interface NSFileManager : NSObject
+ (id)defaultManager;
- (BOOL)createDirectoryAtPath:(NSString*)p withIntermediateDirectories:(BOOL)b attributes:(NSDictionary*)a error:(NSError**)e;
- (BOOL)createFileAtPath:(NSString*)p contents:(NSData*)d attributes:(NSDictionary*)a;
- (BOOL)fileExistsAtPath:(NSString*)p;
- (BOOL)removeItemAtPath:(NSString*)p error:(NSError**)e;
@end
@interface NSFileHandle : NSObject
+ (id)fileHandleForWritingAtPath:(NSString*)p;
- (unsigned long long)seekToEndOfFile;
- (void)closeFile;
@end
@interface NSPipe : NSObject
+ (id)pipe;
- (NSFileHandle*)fileHandleForWriting;
@end
@interface NSTask : NSObject
- (void)setExecutableURL:(NSURL*)u;
- (void)setArguments:(NSArray*)a;
- (void)setStandardInput:(id)f;
- (void)setStandardOutput:(id)f;
- (void)setStandardError:(id)f;
- (BOOL)launchAndReturnError:(NSError**)e;
- (BOOL)isRunning;
- (void)terminate;
@end
@interface NSTimer : NSObject
+ (id)scheduledTimerWithTimeInterval:(NSTimeInterval)t target:(id)target selector:(SEL)s userInfo:(id)info repeats:(BOOL)r;
- (void)invalidate;
@end
@interface NSApplication : NSObject
+ (id)sharedApplication;
- (BOOL)setActivationPolicy:(NSInteger)p;
- (void)setDelegate:(id)d;
- (void)setMainMenu:(NSMenu*)m;
- (void)activateIgnoringOtherApps:(BOOL)b;
- (void)run;
- (void)terminate:(id)sender;
@end
@interface NSView : NSObject
- (id)initWithFrame:(NSRect)r;
- (void)setAutoresizingMask:(NSUInteger)m;
@end
@interface NSWindow : NSObject
- (id)initWithContentRect:(NSRect)r styleMask:(NSUInteger)s backing:(NSUInteger)b defer:(BOOL)d;
- (void)setTitle:(NSString*)s;
- (void)setTitlebarAppearsTransparent:(BOOL)b;
- (void)setContentView:(NSView*)v;
- (void)setMinSize:(NSSize)s;
- (void)setDelegate:(id)d;
- (void)setReleasedWhenClosed:(BOOL)b;
- (BOOL)setFrameUsingName:(NSString*)s;
- (BOOL)setFrameAutosaveName:(NSString*)s;
- (void)center;
- (void)makeKeyAndOrderFront:(id)sender;
@end
@interface NSMenu : NSObject
- (id)initWithTitle:(NSString*)s;
- (void)addItem:(NSMenuItem*)i;
@end
@interface NSMenuItem : NSObject
+ (id)separatorItem;
- (id)initWithTitle:(NSString*)s action:(SEL)a keyEquivalent:(NSString*)k;
- (void)setSubmenu:(NSMenu*)m;
- (void)setTarget:(id)t;
@end
@interface NSOpenPanel : NSWindow
+ (id)openPanel;
- (void)setCanChooseFiles:(BOOL)b;
- (void)setCanChooseDirectories:(BOOL)b;
- (void)setCanCreateDirectories:(BOOL)b;
- (void)setAllowsMultipleSelection:(BOOL)b;
- (void)setPrompt:(NSString*)s;
- (void)setDirectoryURL:(NSURL*)u;
- (NSURL*)URL;
- (void)beginSheetModalForWindow:(NSWindow*)w completionHandler:(void(^)(NSInteger))handler;
@end
@interface NSWorkspace : NSObject
+ (id)sharedWorkspace;
- (BOOL)openURL:(NSURL*)u;
@end
@interface NSPasteboard : NSObject
+ (id)generalPasteboard;
- (NSInteger)clearContents;
- (BOOL)setString:(NSString*)s forType:(NSString*)t;
@end
@interface WKUserContentController : NSObject
- (void)addScriptMessageHandler:(id)h name:(NSString*)n;
@end
@interface WKWebViewConfiguration : NSObject
- (WKUserContentController*)userContentController;
@end
@interface WKFrameInfo : NSObject
- (BOOL)isMainFrame;
@end
@interface WKScriptMessage : NSObject
- (id)body;
- (WKFrameInfo*)frameInfo;
@end
@interface WKNavigationAction : NSObject
- (NSURLRequest*)request;
@end
@interface NSURLRequest (StudioURL)
- (NSURL*)URL;
@end
@interface WKWebView : NSView
- (id)initWithFrame:(NSRect)r configuration:(WKWebViewConfiguration*)c;
- (void)setNavigationDelegate:(id)d;
- (void)setAllowsBackForwardNavigationGestures:(BOOL)b;
- (void)setAllowsLinkPreview:(BOOL)b;
- (id)loadHTMLString:(NSString*)s baseURL:(NSURL*)u;
- (id)loadRequest:(NSURLRequest*)r;
- (void)evaluateJavaScript:(NSString*)s completionHandler:(void(^)(id,NSError*))h;
- (NSURL*)URL;
@end
@protocol NSApplicationDelegate @end
@protocol NSWindowDelegate @end
@protocol WKScriptMessageHandler @end
@protocol WKNavigationDelegate @end
extern NSString* NSHomeDirectory(void);
extern NSString* const NSFilePosixPermissions;
extern NSString* const NSPasteboardTypeString;
#endif
