// Application-menu additions for macOS.
//
// See openfile_darwin.c for why this is a separate translation unit rather
// than a cgo preamble: preambles declare, .c files define.
//
// Gio builds the application menu itself in gio_main (os_macos.m) with only
// Hide and Quit, and offers no API to extend it. The menu exists before the
// first window does, so the standard "About" item is inserted at the top of
// that menu once the first view arrives.

#import <Cocoa/Cocoa.h>

@interface SNVAboutTarget : NSObject
@property(copy) NSString *version;
- (void)showAbout:(id)sender;
@end

@implementation SNVAboutTarget
- (void)showAbout:(id)sender {
    [NSApp activateIgnoringOtherApps:YES];
    NSMutableDictionary *options = [@{
        NSAboutPanelOptionApplicationName : @"SimpleNvimEditor",
        NSAboutPanelOptionApplicationVersion : self.version,
        // Without this the panel appends CFBundleVersion as "(build)".
        NSAboutPanelOptionVersion : @"",
    } mutableCopy];
    // The Dock icon set in icon_darwin.c; an unbundled binary would
    // otherwise get a generic folder.
    NSImage *icon = [NSApp applicationIconImage];
    if (icon != nil) {
        options[NSAboutPanelOptionApplicationIcon] = icon;
    }
    [NSApp orderFrontStandardAboutPanelWithOptions:options];
}
@end

// NSMenuItem.target is weak, so the target must be kept alive here. Its
// presence also marks the menu as already extended: the editor replaces its
// window when a close is vetoed, and each new view calls back in here.
static SNVAboutTarget *snv_about_target = nil;

void snv_install_app_menu(const char *version) {
    NSString *versionString = [NSString stringWithUTF8String:version];
    dispatch_async(dispatch_get_main_queue(), ^{
        if (snv_about_target != nil) {
            return;
        }
        NSMenu *appMenu = [[[NSApp mainMenu] itemAtIndex:0] submenu];
        if (appMenu == nil) {
            return;
        }
        snv_about_target = [SNVAboutTarget new];
        snv_about_target.version = versionString;

        NSMenuItem *about =
            [[NSMenuItem alloc] initWithTitle:@"About SimpleNvimEditor"
                                       action:@selector(showAbout:)
                                keyEquivalent:@""];
        about.target = snv_about_target;
        [appMenu insertItem:about atIndex:0];
        [appMenu insertItem:[NSMenuItem separatorItem] atIndex:1];
        // Outside a .app bundle (`go build && ./simplenvim`) the menu bar
        // would show the executable name instead of CFBundleName.
        [appMenu setTitle:@"SimpleNvimEditor"];
        [[[NSApp mainMenu] itemAtIndex:0] setTitle:@"SimpleNvimEditor"];
    });
}
