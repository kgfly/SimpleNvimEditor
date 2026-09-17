//go:build linux

// XDND target for X11, implemented on a second X connection.
//
// Gio owns the connection that created the window, and XDND's hand-shake
// arrives as ClientMessages with an empty event mask, which X delivers only
// to the client that created the destination window. A second connection
// therefore cannot see them -- unless the target names a proxy. XdndProxy
// exists for exactly this: a source that finds the property re-addresses
// every XDND message to the proxy window, which we do own. See
// https://johnlindal.wixsite.com/xdnd (Technical details, XdndProxy).

#include <X11/Xatom.h>
#include <X11/Xlib.h>
#include <stdlib.h>
#include <string.h>

// Implemented in Go (drop_linux.go, //export snv_onDroppedURIList).
void snv_onDroppedURIList(char *data, int len);

#define SNV_XDND_VERSION 5
#define SNV_MAX_URI_LIST (4 * 1024 * 1024)

struct snv_x11_dnd {
    Display *dpy;
    // target is Gio's window -- what the user aims at, and the id that has
    // to appear in our replies. proxy is ours, and is where the messages
    // actually arrive.
    Window target;
    Window proxy;
    Window source;
    int accepting;

    Atom aware;
    Atom proxyAtom;
    Atom enter;
    Atom position;
    Atom status;
    Atom leave;
    Atom drop;
    Atom finished;
    Atom selection;
    Atom typeList;
    Atom actionCopy;
    Atom uriList;
    Atom incr;
};

static Display *snv_error_display = NULL;
static XErrorHandler snv_prev_error_handler = NULL;

// Xlib's default error handler calls exit(). A drag can race window
// teardown, so a BadWindow on our own connection must not take the editor
// down with it. Handlers are process-global, hence the display check and
// the chain to whatever was installed before.
static int snv_x11_error(Display *dpy, XErrorEvent *err) {
    if (dpy == snv_error_display) {
        return 0;
    }
    if (snv_prev_error_handler != NULL) {
        return snv_prev_error_handler(dpy, err);
    }
    return 0;
}

static void snv_x11_send(struct snv_x11_dnd *d, Window to, Atom message,
                         long l0, long l1, long l2, long l3, long l4) {
    XEvent ev;
    memset(&ev, 0, sizeof(ev));
    ev.xclient.type = ClientMessage;
    ev.xclient.display = d->dpy;
    ev.xclient.window = to;
    ev.xclient.message_type = message;
    ev.xclient.format = 32;
    ev.xclient.data.l[0] = l0;
    ev.xclient.data.l[1] = l1;
    ev.xclient.data.l[2] = l2;
    ev.xclient.data.l[3] = l3;
    ev.xclient.data.l[4] = l4;
    XSendEvent(d->dpy, to, False, NoEventMask, &ev);
    XFlush(d->dpy);
}

static int snv_x11_type_list_has_uris(struct snv_x11_dnd *d, Window source) {
    Atom type = None;
    int format = 0;
    unsigned long count = 0, after = 0;
    unsigned char *data = NULL;
    int found = 0;

    if (XGetWindowProperty(d->dpy, source, d->typeList, 0, 1024, False, XA_ATOM,
                           &type, &format, &count, &after, &data) != Success) {
        return 0;
    }
    if (data != NULL) {
        if (format == 32) {
            Atom *atoms = (Atom *)data;
            for (unsigned long i = 0; i < count; i++) {
                if (atoms[i] == d->uriList) {
                    found = 1;
                    break;
                }
            }
        }
        XFree(data);
    }
    return found;
}

// snv_x11_take_selection reads the converted selection off the proxy window
// and hands the bytes back; the caller frees with XFree.
static unsigned char *snv_x11_take_selection(struct snv_x11_dnd *d,
                                             Atom property,
                                             unsigned long *out_len) {
    Atom type = None;
    int format = 0;
    unsigned long count = 0, after = 0;
    unsigned char *data = NULL;

    *out_len = 0;
    if (XGetWindowProperty(d->dpy, d->proxy, property, 0, 0, False,
                           AnyPropertyType, &type, &format, &count, &after,
                           &data) != Success) {
        return NULL;
    }
    if (data != NULL) {
        XFree(data);
        data = NULL;
    }
    // INCR would need a second, property-notify driven transfer. A uri list
    // never approaches the request limit that triggers it.
    if (after == 0 || type == d->incr || after > SNV_MAX_URI_LIST) {
        return NULL;
    }

    unsigned long longs = (after + 3) / 4;
    if (XGetWindowProperty(d->dpy, d->proxy, property, 0, (long)longs, False,
                           AnyPropertyType, &type, &format, &count, &after,
                           &data) != Success) {
        return NULL;
    }
    if (data == NULL) {
        return NULL;
    }
    if (format != 8) {
        XFree(data);
        return NULL;
    }
    *out_len = count;
    return data;
}

static void snv_x11_finish(struct snv_x11_dnd *d, int accepted) {
    if (d->source != 0) {
        snv_x11_send(d, d->source, d->finished, (long)d->target,
                     accepted ? 1 : 0,
                     accepted ? (long)d->actionCopy : (long)None, 0, 0);
    }
    d->source = 0;
    d->accepting = 0;
}

static void snv_x11_on_client_message(struct snv_x11_dnd *d,
                                      XClientMessageEvent *ev) {
    if (ev->message_type == d->enter) {
        d->source = (Window)ev->data.l[0];
        d->accepting = 0;
        if (ev->data.l[1] & 1) {
            d->accepting = snv_x11_type_list_has_uris(d, d->source);
        } else {
            for (int i = 2; i < 5; i++) {
                if ((Atom)ev->data.l[i] == d->uriList) {
                    d->accepting = 1;
                }
            }
        }
        return;
    }

    if (ev->message_type == d->position) {
        Window source = (Window)ev->data.l[0];
        // An empty rectangle asks the source to keep sending XdndPosition,
        // which costs nothing here: the whole window is one drop zone.
        snv_x11_send(d, source, d->status, (long)d->target,
                     d->accepting ? 1 : 0, 0, 0,
                     d->accepting ? (long)d->actionCopy : (long)None);
        return;
    }

    if (ev->message_type == d->leave) {
        d->source = 0;
        d->accepting = 0;
        return;
    }

    if (ev->message_type == d->drop) {
        d->source = (Window)ev->data.l[0];
        if (!d->accepting) {
            snv_x11_finish(d, 0);
            return;
        }
        XConvertSelection(d->dpy, d->selection, d->uriList, d->selection,
                          d->proxy, (Time)ev->data.l[2]);
        XFlush(d->dpy);
    }
}

static void snv_x11_on_selection(struct snv_x11_dnd *d,
                                 XSelectionEvent *ev) {
    if (ev->property == None) {
        snv_x11_finish(d, 0);
        return;
    }

    unsigned long len = 0;
    unsigned char *data = snv_x11_take_selection(d, ev->property, &len);
    XDeleteProperty(d->dpy, d->proxy, ev->property);
    if (data == NULL) {
        snv_x11_finish(d, 0);
        return;
    }
    snv_onDroppedURIList((char *)data, (int)len);
    XFree(data);
    snv_x11_finish(d, 1);
}

void snv_x11_drop_loop(unsigned long window) {
    struct snv_x11_dnd d;
    memset(&d, 0, sizeof(d));

    d.dpy = XOpenDisplay(NULL);
    if (d.dpy == NULL) {
        return;
    }
    snv_error_display = d.dpy;
    snv_prev_error_handler = XSetErrorHandler(snv_x11_error);

    d.target = (Window)window;
    d.aware = XInternAtom(d.dpy, "XdndAware", False);
    d.proxyAtom = XInternAtom(d.dpy, "XdndProxy", False);
    d.enter = XInternAtom(d.dpy, "XdndEnter", False);
    d.position = XInternAtom(d.dpy, "XdndPosition", False);
    d.status = XInternAtom(d.dpy, "XdndStatus", False);
    d.leave = XInternAtom(d.dpy, "XdndLeave", False);
    d.drop = XInternAtom(d.dpy, "XdndDrop", False);
    d.finished = XInternAtom(d.dpy, "XdndFinished", False);
    d.selection = XInternAtom(d.dpy, "XdndSelection", False);
    d.typeList = XInternAtom(d.dpy, "XdndTypeList", False);
    d.actionCopy = XInternAtom(d.dpy, "XdndActionCopy", False);
    d.uriList = XInternAtom(d.dpy, "text/uri-list", False);
    d.incr = XInternAtom(d.dpy, "INCR", False);

    d.proxy = XCreateSimpleWindow(d.dpy, DefaultRootWindow(d.dpy), -1000, -1000,
                                  1, 1, 0, 0, 0);
    if (d.proxy == 0) {
        XCloseDisplay(d.dpy);
        return;
    }
    XSelectInput(d.dpy, d.proxy, PropertyChangeMask);

    long version = SNV_XDND_VERSION;
    XChangeProperty(d.dpy, d.target, d.aware, XA_ATOM, 32, PropModeReplace,
                    (unsigned char *)&version, 1);
    XChangeProperty(d.dpy, d.proxy, d.aware, XA_ATOM, 32, PropModeReplace,
                    (unsigned char *)&version, 1);
    // The proxy has to name itself, or a conforming source treats the
    // redirection as stale and ignores it.
    XChangeProperty(d.dpy, d.target, d.proxyAtom, XA_WINDOW, 32,
                    PropModeReplace, (unsigned char *)&d.proxy, 1);
    XChangeProperty(d.dpy, d.proxy, d.proxyAtom, XA_WINDOW, 32, PropModeReplace,
                    (unsigned char *)&d.proxy, 1);
    XFlush(d.dpy);

    for (;;) {
        XEvent ev;
        XNextEvent(d.dpy, &ev);
        switch (ev.type) {
        case ClientMessage:
            snv_x11_on_client_message(&d, &ev.xclient);
            break;
        case SelectionNotify:
            snv_x11_on_selection(&d, &ev.xselection);
            break;
        default:
            break;
        }
    }
}
