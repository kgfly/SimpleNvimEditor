//go:build windows && cgo

#define WIN32_LEAN_AND_MEAN
#include <windows.h>
#include <shellapi.h>
#include <stdint.h>
#include <stdlib.h>

// Implemented in Go (drop_windows.go, //export snv_onDroppedFile).
void snv_onDroppedFile(char *path);

static HWND snv_drop_window = NULL;
static WNDPROC snv_previous_wndproc = NULL;

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

    if (message == WM_NCDESTROY && hwnd == snv_drop_window) {
        DragAcceptFiles(hwnd, FALSE);
        SetWindowLongPtrW(hwnd, GWLP_WNDPROC, (LONG_PTR)previous);
        snv_drop_window = NULL;
        snv_previous_wndproc = NULL;
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

    snv_drop_window = hwnd;
    snv_previous_wndproc = (WNDPROC)previous;
    DragAcceptFiles(hwnd, TRUE);
}