# 00016. Wayland-Native Linux GUI with GTK4 and Libadwaita

## Context
Operators on Linux require a native graphical desktop interface to monitor agent backplane feeds, review messages, and inject operator instructions in real time. The Linux client should integrate cleanly with modern Wayland compositors (GNOME, KDE Plasma 6, Hyprland, Sway), avoid legacy X11 baggage, and maintain a lightweight memory footprint.

## Decision
Build the Linux GUI application (`tools/gui-linux`) using **GTK4 + Libadwaita**:
1. **Wayland-Native by Default**: Runs directly over Wayland (`GDK_BACKEND=wayland`) with hardware-accelerated Vulkan/EGL rendering, automatic HiDPI fractional scaling, and gesture navigation.
2. **Desktop Integration via XDG Portals**: Uses FreeDesktop D-Bus interfaces / XDG Desktop Portals for desktop notifications (`org.freedesktop.Notifications`), dark/light theme switching, and StatusNotifierItem (SNI) system tray indicators.
3. **Direct C-ABI Linkage**: Seamlessly links against `libbp.so` (`cmd/libbp-c`) with zero wrapper overhead, updating the GTK event loop from asynchronous backplane callbacks.

## Status
Accepted.

## Consequences
- Native look and feel matching modern Linux developer desktops.
- Low resource usage (~20–40 MB idle RAM) compared to heavy electron or browser-based monitors.
- Complete isolation from deprecated X11 APIs.
