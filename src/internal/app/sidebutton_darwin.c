// Mouse back/forward buttons for macOS.
//
// Gio's -otherMouseDown:/-otherMouseUp: map only buttonNumber 0-2, so the
// back (3) and forward (4) buttons reach Go as a press with no button and
// are dropped. A local event monitor intercepts them before Gio does.

#import <Cocoa/Cocoa.h>

// Implemented in Go (sidebutton_darwin.go, //export snv_onSideButton).
void snv_onSideButton(int button, int press);

void snv_install_side_buttons(void) {
    static dispatch_once_t once;
    dispatch_once(&once, ^{
        NSEventMask mask = NSEventMaskOtherMouseDown | NSEventMaskOtherMouseUp;
        [NSEvent addLocalMonitorForEventsMatchingMask:mask
                                              handler:^NSEvent *(NSEvent *event) {
            NSInteger button = event.buttonNumber;
            if (button != 3 && button != 4) {
                return event;
            }
            snv_onSideButton((int)button, event.type == NSEventTypeOtherMouseDown);
            return nil;
        }];
    });
}
