// Apple Event handler for Finder's "open document" ('odoc') event.
//
// This file contains Objective-C and is compiled as such by the package's
// -x objective-c CFLAGS. Keeping the .c extension prevents cgo from adding
// another -lobjc when Gio already supplies the Objective-C runtime.
//
// This lives in a separate translation unit rather than in the cgo preamble
// of openfile_darwin.go. A preamble is textually prepended to *every* C
// translation unit cgo generates for that package, so any function or ObjC
// class *defined* (not merely declared) there is compiled more than once and
// the link fails with duplicate symbols:
//
//     duplicate symbol '_OBJC_CLASS_$_SNVOpenFileHandler'
//     duplicate symbol '_snv_install_open_file_handler'
//
// That is guaranteed to happen once the same file also uses //export, because
// cgo then emits an extra translation unit for the exported thunks. The rule
// is: preambles declare, separate translation units define.

#import <Cocoa/Cocoa.h>
#import <objc/runtime.h>

// Implemented in Go (openfile_darwin.go, //export snv_onOpenFile).
void snv_onOpenFile(char *path);

static NSDragOperation snv_dragging_entered(id self, SEL command,
                                             id<NSDraggingInfo> sender) {
    NSPasteboard *pasteboard = [sender draggingPasteboard];
    if ([pasteboard canReadObjectForClasses:@[[NSURL class]]
                                    options:@{NSPasteboardURLReadingFileURLsOnlyKey: @YES}]) {
        return NSDragOperationCopy;
    }
    return NSDragOperationNone;
}

static BOOL snv_perform_drag_operation(id self, SEL command,
                                        id<NSDraggingInfo> sender) {
    NSPasteboard *pasteboard = [sender draggingPasteboard];
    NSArray<NSURL *> *urls = [pasteboard
        readObjectsForClasses:@[[NSURL class]]
                      options:@{NSPasteboardURLReadingFileURLsOnlyKey: @YES}];
    NSMutableArray<NSString *> *paths = [[NSMutableArray alloc] init];
    for (NSURL *url in urls) {
        NSString *path = [url path];
        if (path != nil) {
            [paths addObject:path];
        }
    }
    if ([paths count] == 0) {
        return NO;
    }

    dispatch_async(dispatch_get_main_queue(), ^{
        for (NSString *path in paths) {
            snv_onOpenFile((char *)[path UTF8String]);
        }
    });
    return YES;
}

void snv_install_drop_target(uintptr_t viewPointer) {
    NSView *view = (__bridge NSView *)(void *)viewPointer;
    if (view == nil) {
        return;
    }

    dispatch_async(dispatch_get_main_queue(), ^{
        static Class dropViewClass = Nil;
        if (dropViewClass == Nil) {
            // object_getClass, not -class: once anything observes the
            // view with KVO its isa is a hidden NSKVONotifying_ subclass
            // that -class deliberately hides. Subclassing the reported
            // class and then setting it would silently undo that.
            Class gioViewClass = object_getClass(view);
            dropViewClass = objc_allocateClassPair(gioViewClass,
                                                    "SNVDropView", 0);
            if (dropViewClass != Nil) {
                class_addMethod(dropViewClass, @selector(draggingEntered:),
                                (IMP)snv_dragging_entered, "Q@:@");
                class_addMethod(dropViewClass,
                                @selector(performDragOperation:),
                                (IMP)snv_perform_drag_operation, "B@:@");
                objc_registerClassPair(dropViewClass);
            }
        }
        if (dropViewClass != Nil && object_getClass(view) != dropViewClass) {
            object_setClass(view, dropViewClass);
        }
        [view registerForDraggedTypes:@[NSPasteboardTypeFileURL]];
    });
}

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