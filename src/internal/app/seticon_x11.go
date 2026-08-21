//go:build ((linux && !android) || freebsd || openbsd) && !nox11

package editorapp

/*
#cgo linux pkg-config: x11
#cgo freebsd openbsd LDFLAGS: -L/usr/X11R6/lib -L/usr/local/lib -lX11

#include <X11/Xlib.h>
#include <X11/Xatom.h>
#include <stdlib.h>

static void set_net_wm_icon(void *dpy, unsigned long win,
                            unsigned long *data, int n) {
    Atom net_wm_icon = XInternAtom((Display*)dpy, "_NET_WM_ICON", 0);
    Atom cardinal    = XInternAtom((Display*)dpy, "CARDINAL", 0);
    XChangeProperty((Display*)dpy, (Window)win, net_wm_icon, cardinal, 32,
                    PropModeReplace, (unsigned char*)data, n);
    XFlush((Display*)dpy);
}

static void set_net_wm_name(void *dpy, unsigned long win,
                            const char *name, int len) {
    Display *d = (Display*)dpy;
    Atom utf8    = XInternAtom(d, "UTF8_STRING", 0);
    Atom wm_name = XInternAtom(d, "_NET_WM_NAME", 0);
    // Set on the given window and every ancestor up to (but not
    // including) the root, so the WM-managed toplevel is covered
    // regardless of which window the ViewEvent refers to.
    Window root_ret, parent, *children;
    unsigned int nchildren;
    Window cur = (Window)win;
    while (cur) {
        XChangeProperty(d, cur, wm_name, utf8, 8,
                        PropModeReplace, (unsigned char*)name, len);
        XStoreName(d, cur, name);
        if (!XQueryTree(d, cur, &root_ret, &parent, &children, &nchildren))
            break;
        if (children) XFree(children);
        if (parent == root_ret) break;
        cur = parent;
    }
    XFlush(d);
}
*/
import "C"

import (
	"image"
	"unsafe"

	gioapp "gioui.org/app"
)

func setWindowIcon(view any, icon *image.RGBA) {
	ev, ok := view.(gioapp.X11ViewEvent)
	if !ok || !ev.Valid() {
		return
	}
	b := icon.Bounds()
	w, h := b.Dx(), b.Dy()
	n := 2 + w*h
	data := make([]C.ulong, n)
	data[0] = C.ulong(w)
	data[1] = C.ulong(h)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, a := icon.At(x, y).RGBA()
			argb := (uint32(a>>8) << 24) | (uint32(r>>8) << 16) |
				(uint32(g>>8) << 8) | uint32(bl>>8)
			data[2+(y-b.Min.Y)*w+(x-b.Min.X)] = C.ulong(argb)
		}
	}
	C.set_net_wm_icon(ev.Display, C.ulong(ev.Window),
		(*C.ulong)(unsafe.Pointer(&data[0])), C.int(n))
}

func setWindowTitle(view any, title string) {
	ev, ok := view.(gioapp.X11ViewEvent)
	if !ok || !ev.Valid() {
		return
	}
	cs := C.CString(title)
	defer C.free(unsafe.Pointer(cs))
	C.set_net_wm_name(ev.Display, C.ulong(ev.Window), cs, C.int(len(title)))
}
