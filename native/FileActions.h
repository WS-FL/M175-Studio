#import <Cocoa/Cocoa.h>
BOOL StudioCopyFiles(NSArray *paths, NSPasteboard *pasteboard, NSError **error);
BOOL StudioTrashFolder(NSString *path, NSURL **result, NSError **error);
int StudioFileActionMain(int argc, const char *argv[]);
