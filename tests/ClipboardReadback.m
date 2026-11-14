// Independent consumer for native-clipboard.py. Prints file URLs only, never text.
#import <Cocoa/Cocoa.h>
int main(void){@autoreleasepool{
 NSArray *urls=[[NSPasteboard generalPasteboard] readObjectsForClasses:@[[NSURL class]] options:@{NSPasteboardURLReadingFileURLsOnlyKey:@YES}];
 NSMutableArray *paths=[NSMutableArray array];for(NSURL *url in urls)[paths addObject:[url path]];
 NSData *data=[NSJSONSerialization dataWithJSONObject:paths options:0 error:NULL];
 puts([[[NSString alloc] initWithData:data encoding:NSUTF8StringEncoding] UTF8String]);return 0;
}}
