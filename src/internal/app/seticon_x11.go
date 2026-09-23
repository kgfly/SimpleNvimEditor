//go:build ((linux && !android) || freebsd || openbsd) && !nox11

package editorapp

/*
#cgo linux pkg-config: x11
#cgo freebsd openbsd LDFLAGS: -L/usr/X11R6/lib -L/usr/local/lib -lX11

#include <X11/Xlib.h>
#include <X11/Xatom.h>
#include <X11/Xutil.h>
#include <stdlib.h>

static void set_net_wm_icon(void *dpy, unsigned long win,
                            unsigned long *data, int n) {
    Atom net_wm_icon = XInternAtom((Display*)dpy, "_NET_WM_ICON", 0);
    Atom cardinal    = XInternAtom((Display*)dpy, "CARDINAL", 0);
    XChangeProperty((Display*)dpy, (Window)win, net_wm_icon, cardinal, 32,
                    PropModeReplace, (unsigned char*)data, n);
    XFlush((Display*)dpy);
}

static void set_wm_class(void *dpy, unsigned long win, char *name) {
    XClassHint hint = {name, name};
    XSetClassHint((Display*)dpy, (Window)win, &hint);
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
	xdraw "golang.org/x/image/draw"
)

func setWindowIcon(view any, color string) {
	ev, ok := view.(gioapp.X11ViewEvent)
	if !ok || !ev.Valid() {
		return
	}
	data := netWMIcon(decodeIcon(iconPNG(color)))
	C.set_net_wm_icon(ev.Display, C.ulong(ev.Window),
		(*C.ulong)(unsafe.Pointer(&data[0])), C.int(len(data)))

	// Docks that match windows to .desktop files by WM_CLASS show that
	// file's icon, so a color changed after startup needs a new class.
	if id := appID(color); id != gioapp.ID {
		cs := C.CString(id)
		defer C.free(unsafe.Pointer(cs))
		C.set_wm_class(ev.Display, C.ulong(ev.Window), cs)
	}
}

// netWMIcon encodes src as _NET_WM_ICON data at several sizes, so taskbars
// such as IceWM's and xfce4-panel's can pick one instead of scaling.
func netWMIcon(src *image.NRGBA) []C.ulong {
	var data []C.ulong
	for _, size := range []int{16, 32, 48, 64} {
		img := src
		if size != src.Bounds().Dx() {
			img = image.NewNRGBA(image.Rect(0, 0, size, size))
			xdraw.CatmullRom.Scale(img, img.Bounds(), src, src.Bounds(), xdraw.Src, nil)
		}
		b := img.Bounds()
		data = append(data, C.ulong(b.Dx()), C.ulong(b.Dy()))
		for y := b.Min.Y; y < b.Max.Y; y++ {
			for x := b.Min.X; x < b.Max.X; x++ {
				c := img.NRGBAAt(x, y)
				data = append(data, C.ulong(uint32(c.A)<<24|uint32(c.R)<<16|uint32(c.G)<<8|uint32(c.B)))
			}
		}
	}
	return data
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
