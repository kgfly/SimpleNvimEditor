// Dock icon override for macOS.
//
// See openfile_darwin.c for why this is a separate translation unit rather
// than a cgo preamble.

#import <Cocoa/Cocoa.h>

// snv_set_app_icon copies the PNG bytes and applies them on the main
// thread; preferBundle restores the bundle icon if the app has one.
void snv_set_app_icon(const void *data, int len, int preferBundle) {
    NSData *bytes = [NSData dataWithBytes:data length:(NSUInteger)len];
    if (preferBundle && [[NSBundle mainBundle] objectForInfoDictionaryKey:@"CFBundleIconFile"] != nil) {
        bytes = nil;
    }
    dispatch_async(dispatch_get_main_queue(), ^{
        NSImage *image = bytes != nil ? [[NSImage alloc] initWithData:bytes] : nil;
        [NSApp setApplicationIconImage:image];
    });
}
