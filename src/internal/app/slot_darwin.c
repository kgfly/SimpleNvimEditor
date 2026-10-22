// Instance slot claim for macOS; see slot.go.

#include <CoreFoundation/CoreFoundation.h>

// snv_claim_slot returns the named local port if this process now owns it,
// else NULL. The port lives until snv_release_slot or the process ends.
void *snv_claim_slot(const char *name) {
    CFStringRef s = CFStringCreateWithCString(NULL, name, kCFStringEncodingUTF8);
    if (s == NULL) {
        return NULL;
    }
    CFMessagePortRef port = CFMessagePortCreateLocal(NULL, s, NULL, NULL, NULL);
    CFRelease(s);
    return (void *)port;
}

// snv_release_slot unregisters a port from snv_claim_slot, freeing its name.
void snv_release_slot(void *port) {
    CFMessagePortInvalidate((CFMessagePortRef)port);
    CFRelease((CFMessagePortRef)port);
}
