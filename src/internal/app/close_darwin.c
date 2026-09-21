// Close/quit interception for macOS.
//
// See openfile_darwin.c for why this is a separate translation unit rather
// than a cgo preamble: preambles declare, .c files define.
//
// Cmd+Q reaches us as -[NSApplication terminate:] (the Quit menu item's key
// equivalent, installed by Gio in os_macos.m), and the close button and
// Cmd+W reach us as -[NSWindow performClose:]. Neither is something Gio
// exposes as a cancellable event, so the two AppKit veto points are added
// directly to the classes Gio already uses for its delegates. Adding a
// method is additive -- Gio implements neither selector -- and keeps Gio's
// own delegate objects in place.

#import <Cocoa/Cocoa.h>
#import <objc/runtime.h>

// Implemented in Go (close_darwin.go, //export snv_onCloseRequest).
void snv_onCloseRequest(void);

// snv_close_allowed is set once the editor has decided the window really
// should go away (Nvim exited). Without it, the programmatic close in
// pumpRedraw would be vetoed by our own hook and the window would never
// shut. Set from a Go goroutine, read on the main thread, hence atomic.
static int snv_close_allowed = 0;

static BOOL snv_is_close_allowed(void) {
    return __atomic_load_n(&snv_close_allowed, __ATOMIC_SEQ_CST) != 0;
}

static NSApplicationTerminateReply snv_should_terminate(id self, SEL cmd,
                                                        NSApplication *sender) {
    if (snv_is_close_allowed()) {
        return NSTerminateNow;
    }
    snv_onCloseRequest();
    return NSTerminateCancel;
}

static BOOL snv_window_should_close(id self, SEL cmd, NSWindow *sender) {
    if (snv_is_close_allowed()) {
        return YES;
    }
    snv_onCloseRequest();
    return NO;
}

// Objective-C method signatures for the two selectors. The return-type
// character is filled in from @encode at install time rather than written
// out, because BOOL is _Bool on arm64 and signed char on x86_64. The
// buffers are static because they must outlive the class_addMethod call.
static char snv_terminate_types[] = "?@:@";
static char snv_should_close_types[] = "?@:@";

// snv_add_method installs imp for selector on cls whether or not the class
// already responds: class_addMethod refuses to overwrite, so an existing
// implementation (a future Gio that grows one) is replaced instead.
static void snv_add_method(Class cls, SEL selector, IMP imp, const char *types) {
    if (cls == Nil) {
        return;
    }
    if (!class_addMethod(cls, selector, imp, types)) {
        class_replaceMethod(cls, selector, imp, types);
    }
}

void snv_install_close_handler(uintptr_t viewPointer) {
    NSView *view = (__bridge NSView *)(void *)viewPointer;
    if (view == nil) {
        return;
    }

    snv_terminate_types[0] = @encode(NSApplicationTerminateReply)[0];
    snv_should_close_types[0] = @encode(BOOL)[0];

    // Deferred to the main queue so that the window and its delegate are
    // both wired up: Gio sets window.delegate after -setContentView:.
    dispatch_async(dispatch_get_main_queue(), ^{
        id appDelegate = [NSApp delegate];
        if (appDelegate != nil) {
            snv_add_method(object_getClass(appDelegate),
                           @selector(applicationShouldTerminate:),
                           (IMP)snv_should_terminate, snv_terminate_types);
        }

        id windowDelegate = [[view window] delegate];
        Class windowDelegateClass = windowDelegate != nil
            ? object_getClass(windowDelegate)
            : objc_getClass("GioWindowDelegate");
        snv_add_method(windowDelegateClass, @selector(windowShouldClose:),
                       (IMP)snv_window_should_close, snv_should_close_types);
    });
}

void snv_allow_close(void) {
    __atomic_store_n(&snv_close_allowed, 1, __ATOMIC_SEQ_CST);
}
