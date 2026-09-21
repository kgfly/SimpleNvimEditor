//go:build windows && cgo

#define WIN32_LEAN_AND_MEAN
#include <windows.h>
#include <shellapi.h>
#include <stdint.h>
#include <stdlib.h>

// Implemented in Go (drop_windows.go, //export snv_onDroppedFile).
void snv_onDroppedFile(char *path);

// Implemented in Go (close_windows.go, //export snv_onCloseRequest).
void snv_onCloseRequest(void);

static HWND snv_drop_window = NULL;
static WNDPROC snv_previous_wndproc = NULL;

// snv_close_allowed is set once the editor has decided the window really
// should go away (Nvim exited). Until then WM_CLOSE is swallowed so that
// Nvim can ask about unsaved buffers; without the flag the programmatic
// close in pumpRedraw would be swallowed too and the window would never
// shut. Set from a Go goroutine, read on the UI thread, hence interlocked.
static volatile LONG snv_close_allowed = 0;

void snv_allow_close(void) {
    InterlockedExchange(&snv_close_allowed, 1);
}

static void snv_send_dropped_file(HDROP drop, UINT index) {
    UINT wideLength = DragQueryFileW(drop, index, NULL, 0);
    if (wideLength == 0) {
        return;
    }

    WCHAR *widePath = malloc((wideLength + 1) * sizeof(WCHAR));
    if (widePath == NULL) {
        return;
    }
    if (DragQueryFileW(drop, index, widePath, wideLength + 1) == 0) {
        free(widePath);
        return;
    }

    int utf8Length = WideCharToMultiByte(CP_UTF8, 0, widePath, -1,
                                         NULL, 0, NULL, NULL);
    if (utf8Length > 0) {
        char *utf8Path = malloc((size_t)utf8Length);
        if (utf8Path != NULL) {
            if (WideCharToMultiByte(CP_UTF8, 0, widePath, -1, utf8Path,
                                    utf8Length, NULL, NULL) > 0) {
                snv_onDroppedFile(utf8Path);
            }
            free(utf8Path);
        }
    }
    free(widePath);
}

static LRESULT CALLBACK snv_drop_wndproc(HWND hwnd, UINT message,
                                         WPARAM wParam, LPARAM lParam) {
    WNDPROC previous = snv_previous_wndproc;

    if (message == WM_DROPFILES) {
        HDROP drop = (HDROP)wParam;
        UINT count = DragQueryFileW(drop, 0xFFFFFFFF, NULL, 0);
        for (UINT index = 0; index < count; index++) {
            snv_send_dropped_file(drop, index);
        }
        DragFinish(drop);
        return 0;
    }

    // Alt+F4, the title-bar close button and the taskbar's Close all arrive
    // as WM_CLOSE (Alt+F4 via SC_CLOSE, which DefWindowProc turns into one).
    // Gio has no close-request event and destroys the window as soon as it
    // sees WM_DESTROY, so cancel here and let Nvim decide instead.
    if (message == WM_CLOSE &&
        InterlockedCompareExchange(&snv_close_allowed, 0, 0) == 0) {
        snv_onCloseRequest();
        return 0;
    }

    if (message == WM_NCDESTROY && hwnd == snv_drop_window) {
        DragAcceptFiles(hwnd, FALSE);
        SetWindowLongPtrW(hwnd, GWLP_WNDPROC, (LONG_PTR)previous);
        snv_drop_window = NULL;
        snv_previous_wndproc = NULL;
    }

    // CallWindowProcW faults on a null procedure, so never chain into one:
    // Gio's own procedure is unreachable for as long as it takes
    // snv_install_drop_target to publish it.
    if (previous == NULL) {
        return DefWindowProcW(hwnd, message, wParam, lParam);
    }
    return CallWindowProcW(previous, hwnd, message, wParam, lParam);
}

void snv_install_drop_target(uintptr_t windowPointer) {
    HWND hwnd = (HWND)windowPointer;
    if (hwnd == NULL || hwnd == snv_drop_window) {
        return;
    }

    if (snv_drop_window != NULL && snv_previous_wndproc != NULL) {
        DragAcceptFiles(snv_drop_window, FALSE);
        SetWindowLongPtrW(snv_drop_window, GWLP_WNDPROC,
                          (LONG_PTR)snv_previous_wndproc);
        snv_drop_window = NULL;
        snv_previous_wndproc = NULL;
    }

    SetLastError(0);
    LONG_PTR previous = SetWindowLongPtrW(hwnd, GWLP_WNDPROC,
                                          (LONG_PTR)snv_drop_wndproc);
    if (previous == 0 && GetLastError() != 0) {
        return;
    }

    snv_previous_wndproc = (WNDPROC)previous;
    snv_drop_window = hwnd;
    DragAcceptFiles(hwnd, TRUE);
}
