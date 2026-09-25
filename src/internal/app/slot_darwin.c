// Instance slot claim for macOS; see slot.go.

#include <CoreFoundation/CoreFoundation.h>

// snv_claim_slot returns 1 if this process now owns the named local port.
// The port is never released, so it lives as long as the process.
int snv_claim_slot(const char *name) {
    CFStringRef s = CFStringCreateWithCString(NULL, name, kCFStringEncodingUTF8);
    if (s == NULL) {
        return 0;
    }
    CFMessagePortRef port = CFMessagePortCreateLocal(NULL, s, NULL, NULL, NULL);
    CFRelease(s);
    return port != NULL;
}
