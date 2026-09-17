//go:build linux

// Wayland drop target, implemented as a second wl_data_device on the
// connection Gio already owns.
//
// A drag is delivered to the client that owns the focused surface, and the
// compositor sends enter/motion/drop to *every* wl_data_device that client
// has bound for the seat (wlroots: drag_set_focus and drag_drop both
// wl_resource_for_each over seat_client->data_devices). Binding a second
// one therefore gets us our own wl_data_offer alongside Gio's, which it
// leaves untouched -- Gio's DnD handlers are empty and it never calls
// wl_data_offer.accept, so ours is the only acceptance the source sees.
//
// The proxies live on a private wl_event_queue so this thread can dispatch
// them while Gio keeps dispatching the default queue; that split is the
// supported libwayland threading model.

#define _GNU_SOURCE

#include <fcntl.h>
#include <poll.h>
#include <stdint.h>
#include <stdlib.h>
#include <string.h>
#include <unistd.h>
#include <wayland-client.h>

// Implemented in Go (drop_linux.go, //export snv_onDroppedURIList).
void snv_onDroppedURIList(char *data, int len);

#define SNV_URI_LIST "text/uri-list"
#define SNV_MAX_URI_LIST (4 * 1024 * 1024)
#define SNV_READ_TIMEOUT_MS 3000

struct snv_wl_dnd {
    struct wl_display *display;
    struct wl_surface *surface;
    struct wl_event_queue *queue;
    struct wl_seat *seat;
    struct wl_data_device_manager *manager;
    struct wl_data_device *device;
    uint32_t manager_version;

    struct wl_data_offer *offer;
    int offer_has_uris;
    int entered;
};

static void snv_wl_forget_offer(struct snv_wl_dnd *d) {
    if (d->offer != NULL) {
        wl_data_offer_destroy(d->offer);
        d->offer = NULL;
    }
    d->offer_has_uris = 0;
    d->entered = 0;
}

static void snv_wl_offer_mime(void *data, struct wl_data_offer *offer,
                              const char *mime) {
    struct snv_wl_dnd *d = data;
    if (offer == d->offer && strcmp(mime, SNV_URI_LIST) == 0) {
        d->offer_has_uris = 1;
    }
}

static void snv_wl_offer_source_actions(void *data,
                                        struct wl_data_offer *offer,
                                        uint32_t actions) {
    (void)data;
    (void)offer;
    (void)actions;
}

static void snv_wl_offer_action(void *data, struct wl_data_offer *offer,
                                uint32_t action) {
    (void)data;
    (void)offer;
    (void)action;
}

static const struct wl_data_offer_listener snv_wl_offer_listener = {
    .offer = snv_wl_offer_mime,
    .source_actions = snv_wl_offer_source_actions,
    .action = snv_wl_offer_action,
};

static void snv_wl_device_data_offer(void *data, struct wl_data_device *dev,
                                     struct wl_data_offer *offer) {
    struct snv_wl_dnd *d = data;
    (void)dev;
    snv_wl_forget_offer(d);
    d->offer = offer;
    wl_data_offer_add_listener(offer, &snv_wl_offer_listener, d);
}

static void snv_wl_device_enter(void *data, struct wl_data_device *dev,
                                uint32_t serial, struct wl_surface *surface,
                                wl_fixed_t x, wl_fixed_t y,
                                struct wl_data_offer *offer) {
    struct snv_wl_dnd *d = data;
    (void)dev;
    (void)x;
    (void)y;

    d->entered = 0;
    if (offer == NULL) {
        return;
    }
    d->offer = offer;
    if (surface != d->surface || !d->offer_has_uris) {
        wl_data_offer_accept(offer, serial, NULL);
        return;
    }
    d->entered = 1;
    wl_data_offer_accept(offer, serial, SNV_URI_LIST);
    if (d->manager_version >= 3) {
        wl_data_offer_set_actions(offer,
                                  WL_DATA_DEVICE_MANAGER_DND_ACTION_COPY,
                                  WL_DATA_DEVICE_MANAGER_DND_ACTION_COPY);
    }
    wl_display_flush(d->display);
}

static void snv_wl_device_leave(void *data, struct wl_data_device *dev) {
    struct snv_wl_dnd *d = data;
    (void)dev;
    snv_wl_forget_offer(d);
}

static void snv_wl_device_motion(void *data, struct wl_data_device *dev,
                                 uint32_t time, wl_fixed_t x, wl_fixed_t y) {
    (void)data;
    (void)dev;
    (void)time;
    (void)x;
    (void)y;
}

static char *snv_wl_read_all(int fd, size_t *out_len) {
    char *buf = NULL;
    size_t len = 0, cap = 0;

    for (;;) {
        struct pollfd pfd;
        pfd.fd = fd;
        pfd.events = POLLIN;
        pfd.revents = 0;
        int ready = poll(&pfd, 1, SNV_READ_TIMEOUT_MS);
        if (ready <= 0) {
            break;
        }
        if (cap - len < 4096) {
            size_t next = cap == 0 ? 8192 : cap * 2;
            char *grown = realloc(buf, next);
            if (grown == NULL) {
                break;
            }
            buf = grown;
            cap = next;
        }
        ssize_t n = read(fd, buf + len, cap - len);
        if (n <= 0) {
            break;
        }
        len += (size_t)n;
        if (len >= SNV_MAX_URI_LIST) {
            break;
        }
    }

    *out_len = len;
    return buf;
}

static void snv_wl_device_drop(void *data, struct wl_data_device *dev) {
    struct snv_wl_dnd *d = data;
    (void)dev;

    if (!d->entered || d->offer == NULL) {
        return;
    }

    int fds[2];
    if (pipe2(fds, O_CLOEXEC) != 0) {
        snv_wl_forget_offer(d);
        return;
    }
    wl_data_offer_receive(d->offer, SNV_URI_LIST, fds[1]);
    close(fds[1]);
    // The source only learns it has to write once the request is on the
    // wire, and this thread is about to stop dispatching to read it.
    wl_display_flush(d->display);

    size_t len = 0;
    char *buf = snv_wl_read_all(fds[0], &len);
    close(fds[0]);

    if (d->manager_version >= 3) {
        wl_data_offer_finish(d->offer);
    }
    snv_wl_forget_offer(d);
    wl_display_flush(d->display);

    if (buf != NULL) {
        if (len > 0) {
            snv_onDroppedURIList(buf, (int)len);
        }
        free(buf);
    }
}

static void snv_wl_device_selection(void *data, struct wl_data_device *dev,
                                    struct wl_data_offer *offer) {
    struct snv_wl_dnd *d = data;
    (void)dev;
    // Clipboard is Gio's business; this device only exists for drops.
    if (offer == NULL) {
        return;
    }
    if (offer == d->offer) {
        d->offer = NULL;
        d->offer_has_uris = 0;
        d->entered = 0;
    }
    wl_data_offer_destroy(offer);
}

static const struct wl_data_device_listener snv_wl_device_listener = {
    .data_offer = snv_wl_device_data_offer,
    .enter = snv_wl_device_enter,
    .leave = snv_wl_device_leave,
    .motion = snv_wl_device_motion,
    .drop = snv_wl_device_drop,
    .selection = snv_wl_device_selection,
};

static void snv_wl_registry_global(void *data, struct wl_registry *registry,
                                   uint32_t name, const char *interface,
                                   uint32_t version) {
    struct snv_wl_dnd *d = data;

    if (d->seat == NULL && strcmp(interface, "wl_seat") == 0) {
        uint32_t bind = version < 5 ? version : 5;
        d->seat = wl_registry_bind(registry, name, &wl_seat_interface, bind);
        return;
    }
    if (d->manager == NULL &&
        strcmp(interface, "wl_data_device_manager") == 0) {
        uint32_t bind = version < 3 ? version : 3;
        d->manager_version = bind;
        d->manager = wl_registry_bind(registry, name,
                                      &wl_data_device_manager_interface, bind);
    }
}

static void snv_wl_registry_global_remove(void *data,
                                          struct wl_registry *registry,
                                          uint32_t name) {
    (void)data;
    (void)registry;
    (void)name;
}

static const struct wl_registry_listener snv_wl_registry_listener = {
    .global = snv_wl_registry_global,
    .global_remove = snv_wl_registry_global_remove,
};

void snv_wl_drop_loop(void *display, void *surface) {
    struct snv_wl_dnd d;
    memset(&d, 0, sizeof(d));
    d.display = (struct wl_display *)display;
    d.surface = (struct wl_surface *)surface;

    d.queue = wl_display_create_queue(d.display);
    if (d.queue == NULL) {
        return;
    }
    struct wl_registry *registry = wl_display_get_registry(d.display);
    if (registry == NULL) {
        return;
    }
    // Everything bound through this registry inherits its queue, so this one
    // assignment keeps all of our proxies off Gio's default queue.
    wl_proxy_set_queue((struct wl_proxy *)registry, d.queue);
    wl_registry_add_listener(registry, &snv_wl_registry_listener, &d);

    if (wl_display_roundtrip_queue(d.display, d.queue) < 0) {
        return;
    }
    if (d.seat == NULL || d.manager == NULL) {
        return;
    }

    d.device = wl_data_device_manager_get_data_device(d.manager, d.seat);
    if (d.device == NULL) {
        return;
    }
    wl_data_device_add_listener(d.device, &snv_wl_device_listener, &d);
    wl_display_flush(d.display);

    while (wl_display_dispatch_queue(d.display, d.queue) >= 0) {
    }
}
