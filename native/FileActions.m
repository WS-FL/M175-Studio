#import "FileActions.h"

static BOOL InvalidFile(NSError **error, NSString *message) {
    if(error)*error=[NSError errorWithDomain:@"M175FileAction" code:1 userInfo:@{NSLocalizedDescriptionKey:message}];
    return NO;
}
BOOL StudioCopyFiles(NSArray *paths, NSPasteboard *pasteboard, NSError **error) {
    if(![paths count])return InvalidFile(error,@"No files selected");
    NSMutableArray *urls=[NSMutableArray array];
    for(NSString *path in paths){
        NSDictionary *attrs=[[NSFileManager defaultManager] attributesOfItemAtPath:path error:error];
        if(![path isAbsolutePath]||![[attrs objectForKey:NSFileType] isEqualToString:NSFileTypeRegular])return InvalidFile(error,@"The selected file is unavailable");
        [urls addObject:[NSURL fileURLWithPath:path]];
    }
    // NSURL objects publish public.file-url, so Finder and other Mac apps can
    // paste the actual files rather than receiving plain-text filesystem paths.
    [pasteboard clearContents];
    if(![pasteboard writeObjects:urls])return InvalidFile(error,@"Could not copy files to the clipboard");
    return YES;
}
BOOL StudioTrashFolder(NSString *path, NSURL **result, NSError **error) {
    NSDictionary *attrs=[[NSFileManager defaultManager] attributesOfItemAtPath:path error:error];
    if(![path isAbsolutePath]||![[path lastPathComponent] hasPrefix:@"scan-"]||![[attrs objectForKey:NSFileType] isEqualToString:NSFileTypeDirectory])return InvalidFile(error,@"The selected document folder is unavailable");
    return [[NSFileManager defaultManager] trashItemAtURL:[NSURL fileURLWithPath:path] resultingItemURL:result error:error];
}
int StudioFileActionMain(int argc, const char *argv[]) {
    if(argc<2)return -1;
    NSString *action=[NSString stringWithUTF8String:argv[1]];
    if(![action isEqualToString:@"--copy-files"]&&![action isEqualToString:@"--trash-folder"])return -1;
    NSError *error=nil;BOOL ok=NO;
    if([action isEqualToString:@"--copy-files"]){
        NSMutableArray *paths=[NSMutableArray array];
        for(int i=2;i<argc;i++)[paths addObject:[NSString stringWithUTF8String:argv[i]]];
        ok=StudioCopyFiles(paths,[NSPasteboard generalPasteboard],&error);
    }else if(argc==3){ok=StudioTrashFolder([NSString stringWithUTF8String:argv[2]],NULL,&error);}
    if(!ok)fprintf(stderr,"%s\n",[[error localizedDescription] UTF8String]?:"Invalid file operation");
    return ok?0:1;
}
