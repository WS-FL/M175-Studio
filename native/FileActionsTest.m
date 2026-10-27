// Isolated fixtures and a named pasteboard; never reads or replaces user clipboard.
#import "FileActions.h"
#define CHECK(condition) do{if(!(condition)){fprintf(stderr,"Failed line %d: %s\n",__LINE__,#condition);return 1;}}while(0)
int main(void){@autoreleasepool{
 NSFileManager *fm=[NSFileManager defaultManager];NSError *error=nil;
 NSString *root=[NSTemporaryDirectory() stringByAppendingPathComponent:[[NSUUID UUID] UUIDString]];
 NSString *dir=[root stringByAppendingPathComponent:@"scan-file-actions-test"];
 CHECK([fm createDirectoryAtPath:dir withIntermediateDirectories:YES attributes:nil error:&error]);
 NSString *pdf=[dir stringByAppendingPathComponent:@"研究 Report.pdf"],*jpeg=[dir stringByAppendingPathComponent:@"page-001.jpg"],*second=[dir stringByAppendingPathComponent:@"page-002.jpg"];
 NSData *content=[@"disposable fixture" dataUsingEncoding:NSUTF8StringEncoding];
 for(NSString *path in @[pdf,jpeg,second])CHECK([content writeToFile:path atomically:YES]);
 NSPasteboard *board=[NSPasteboard pasteboardWithName:[@"M175StudioTest-" stringByAppendingString:[[NSUUID UUID] UUIDString]]];
 CHECK(StudioCopyFiles(@[pdf],board,&error));
 NSArray *urls=[board readObjectsForClasses:@[[NSURL class]] options:@{NSPasteboardURLReadingFileURLsOnlyKey:@YES}];
 CHECK([urls count]==1&&[[[urls objectAtIndex:0] path] isEqualToString:pdf]);
 CHECK(StudioCopyFiles(@[jpeg,second],board,&error));
 urls=[board readObjectsForClasses:@[[NSURL class]] options:@{NSPasteboardURLReadingFileURLsOnlyKey:@YES}];CHECK([urls count]==2);
 CHECK(!StudioCopyFiles(@[dir],board,&error));CHECK(!StudioCopyFiles(@[],board,&error));
 CHECK(!StudioTrashFolder(pdf,NULL,&error));
 NSString *link=[root stringByAppendingPathComponent:@"scan-symlink"];CHECK([fm createSymbolicLinkAtPath:link withDestinationPath:dir error:&error]);CHECK(!StudioTrashFolder(link,NULL,&error));
 NSURL *trashed=nil;CHECK(StudioTrashFolder(dir,&trashed,&error));CHECK(trashed!=nil);CHECK(![fm fileExistsAtPath:dir]);
 for(NSString *name in @[@"研究 Report.pdf",@"page-001.jpg",@"page-002.jpg"])CHECK([[NSData dataWithContentsOfURL:[trashed URLByAppendingPathComponent:name]] isEqualToData:content]);
 // Remove only this test's returned Trash URL and private temporary directory.
 CHECK([fm removeItemAtURL:trashed error:&error]);CHECK([fm removeItemAtPath:root error:&error]);[board releaseGlobally];
 puts("PASS: native file URL clipboard, multiple JPEGs, validation and recoverable Trash contents");return 0;
}}
