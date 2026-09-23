// Dock icon override for macOS.
//
// See openfile_darwin.c for why this is a separate translation unit rather
// than a cgo preamble.

#import <Cocoa/Cocoa.h>

// snv_set_app_icon copies the PNG bytes and applies them on the main
// thread; a NULL/0 image restores the bundle icon.
void snv_set_app_icon(const void *data, int len) {
    NSData *bytes = len > 0 ? [NSData dataWithBytes:data length:(NSUInteger)len] : nil;
    dispatch_async(dispatch_get_main_queue(), ^{
        NSImage *image = bytes != nil ? [[NSImage alloc] initWithData:bytes] : nil;
        [NSApp setApplicationIconImage:image];
    });
}
