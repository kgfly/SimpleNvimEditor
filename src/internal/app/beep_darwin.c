// Beep suppression for macOS.
//
// See openfile_darwin.c for why this is a separate translation unit rather
// than a cgo preamble.
//
// Gio's -[GioView keyDown:] runs every key press through
// -interpretKeyEvents:, which maps Emacs-style chords (Ctrl+F -> moveForward:,
// Ctrl+B -> moveBackward:, ...) to Cocoa text-editing selectors and hands
// them to -doCommandBySelector:. Gio only consumes the selectors it knows
// about and forwards the rest to super, and NSResponder beeps when nothing
// in the responder chain implements the action. The key itself still
// reaches Nvim via Gio's key.Event, so the beep is pure noise: replace
// GioView's -doCommandBySelector: with one that never falls through to the
// beeping default.

#import <Cocoa/Cocoa.h>
#import <objc/message.h>
#import <objc/runtime.h>
#include <stdbool.h>
#include <stdint.h>

// Exported by gioui.org/app (os_macos.go, //export gio_onCommandBySelector).
extern bool gio_onCommandBySelector(uintptr_t handle);

static void snv_do_command_by_selector(id self, SEL cmd, SEL action) {
    uintptr_t handle =
        ((uintptr_t (*)(id, SEL))objc_msgSend)(self, @selector(handle));
    if (gio_onCommandBySelector(handle)) {
        return;
    }
    // Give the responder chain a chance at the action, silently: unlike
    // -doCommandBySelector:, -tryToPerform:with: does not beep on failure.
    [self tryToPerform:action with:nil];
}

void snv_install_beep_suppressor(uintptr_t viewPointer) {
    NSView *view = (__bridge NSView *)(void *)viewPointer;
    if (view == nil) {
        return;
    }
    Class cls = object_getClass(view);
    if (![view respondsToSelector:@selector(handle)]) {
        return;
    }
    Method m = class_getInstanceMethod(cls, @selector(doCommandBySelector:));
    if (m == NULL) {
        return;
    }
    class_replaceMethod(cls, @selector(doCommandBySelector:),
                        (IMP)snv_do_command_by_selector,
                        method_getTypeEncoding(m));
}
