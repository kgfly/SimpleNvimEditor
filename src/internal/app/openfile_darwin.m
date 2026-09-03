// Apple Event handler for Finder's "open document" ('odoc') event.
//
// This lives in a real .m file rather than in the cgo preamble of
// openfile_darwin.go. A preamble is textually prepended to *every* C
// translation unit cgo generates for that package, so any function or ObjC
// class *defined* (not merely declared) there is compiled more than once and
// the link fails with duplicate symbols:
//
//     duplicate symbol '_OBJC_CLASS_$_SNVOpenFileHandler'
//     duplicate symbol '_snv_install_open_file_handler'
//
// That is guaranteed to happen once the same file also uses //export, because
// cgo then emits an extra translation unit for the exported thunks. The rule
// is: preambles declare, .m/.c files define.

#import <Cocoa/Cocoa.h>

// Implemented in Go (openfile_darwin.go, //export snv_onOpenFile).
void snv_onOpenFile(char *path);

// SNVOpenFileHandler receives the 'odoc' (kAEOpenDocuments) Apple Event,
// which is what Finder sends when a file is opened with this app.
//
// Why an Apple Event handler rather than the NSApplicationDelegate method:
// Gio owns the delegate (GioAppDelegate in os_macos.m) and implements only
// application:openURLs:, which fires for registered URL *schemes*, not for
// file opens. Replacing Gio's delegate would fight the toolkit for
// ownership and break on upgrade. Registering directly with
// NSAppleEventManager is additive and leaves Gio untouched: AppKit's
// built-in 'odoc' handler is merely what would otherwise forward to
// application:openFile:, so claiming that one event changes nothing else.
@interface SNVOpenFileHandler : NSObject
@end

@implementation SNVOpenFileHandler

- (void)handleOpenDocs:(NSAppleEventDescriptor *)event
        withReplyEvent:(NSAppleEventDescriptor *)reply {
    NSAppleEventDescriptor *list = [event paramDescriptorForKeyword:keyDirectObject];
    if (list == nil) {
        return;
    }
    // Apple Event descriptor lists are 1-based.
    for (NSInteger i = 1; i <= [list numberOfItems]; i++) {
        NSAppleEventDescriptor *item = [list descriptorAtIndex:i];
        NSString *path = nil;

        // Finder sends typeFileURL; older senders use an alias/FSRef,
        // which coercing to typeFileURL normalises.
        NSAppleEventDescriptor *urlDesc = [item coerceToDescriptorType:typeFileURL];
        if (urlDesc != nil) {
            NSString *s = [[NSString alloc] initWithData:[urlDesc data]
                                                encoding:NSUTF8StringEncoding];
            path = [[NSURL URLWithString:s] path];
        }
        if (path == nil) {
            path = [item stringValue];
        }
        if (path != nil) {
            snv_onOpenFile((char *)[path UTF8String]);
        }
    }
}

@end

static SNVOpenFileHandler *snvHandler = nil;

void snv_install_open_file_handler(void) {
    // NSAppleEventManager is not thread-safe and must be registered
    // against the main run loop.
    dispatch_async(dispatch_get_main_queue(), ^{
        if (snvHandler != nil) {
            return;
        }
        snvHandler = [[SNVOpenFileHandler alloc] init];
        [[NSAppleEventManager sharedAppleEventManager]
            setEventHandler:snvHandler
                andSelector:@selector(handleOpenDocs:withReplyEvent:)
              forEventClass:kCoreEventClass
                 andEventID:kAEOpenDocuments];
    });
}
