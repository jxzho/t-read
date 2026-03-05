import Cocoa
import WebKit

let targetURL = CommandLine.arguments.count > 1 ? CommandLine.arguments[1] : ""
let theme = CommandLine.arguments.count > 2 ? CommandLine.arguments[2].lowercased() : "dark"
let skinScrollbar = CommandLine.arguments.count > 3 ? ((CommandLine.arguments[3] as NSString).boolValue) : true
let zoomStatePath = CommandLine.arguments.count > 4 ? CommandLine.arguments[4] : ""

func clamp(_ value: CGFloat, minValue: CGFloat, maxValue: CGFloat) -> CGFloat {
    return min(max(value, minValue), maxValue)
}

func loadZoom(path: String, minValue: CGFloat, maxValue: CGFloat) -> CGFloat {
    guard !path.isEmpty else { return 1.0 }
    guard let raw = try? String(contentsOfFile: path, encoding: .utf8) else { return 1.0 }
    let trimmed = raw.trimmingCharacters(in: .whitespacesAndNewlines)
    guard let val = Double(trimmed) else { return 1.0 }
    return clamp(CGFloat(val), minValue: minValue, maxValue: maxValue)
}

func saveZoom(_ value: CGFloat, path: String) {
    guard !path.isEmpty else { return }
    let dir = (path as NSString).deletingLastPathComponent
    if !dir.isEmpty {
        try? FileManager.default.createDirectory(atPath: dir, withIntermediateDirectories: true)
    }
    let text = String(format: "%.2f", Double(value))
    try? text.write(toFile: path, atomically: true, encoding: .utf8)
}

func makeInjectedCSS(theme: String, skinScrollbar: Bool) -> String {
    let scrollbarDark = """
    ::-webkit-scrollbar { width: 10px; height: 10px; }
    ::-webkit-scrollbar-track { background: #1a1a1a !important; }
    ::-webkit-scrollbar-thumb { background: #555 !important; border-radius: 8px; }
    ::-webkit-scrollbar-thumb:hover { background: #777 !important; }
    """
    let scrollbarLight = """
    ::-webkit-scrollbar { width: 10px; height: 10px; }
    ::-webkit-scrollbar-track { background: #f2f2f2 !important; }
    ::-webkit-scrollbar-thumb { background: #c1c1c1 !important; border-radius: 8px; }
    ::-webkit-scrollbar-thumb:hover { background: #a6a6a6 !important; }
    """

    if !skinScrollbar { return "" }
    let scrollbar = theme == "light" ? scrollbarLight : scrollbarDark
    return scrollbar
}

class AppDelegate: NSObject, NSApplicationDelegate, WKUIDelegate {
    var window: NSWindow!
    var webView: WKWebView!
    var zoomMonitor: Any?
    var currentZoom: CGFloat = 1.0
    let minZoom: CGFloat = 0.7
    let maxZoom: CGFloat = 1.8
    let zoomStep: CGFloat = 0.1

    func applicationDidFinishLaunching(_ notification: Notification) {
        let frame = NSRect(x: 0, y: 0, width: 1280, height: 860)
        window = NSWindow(
            contentRect: frame,
            styleMask: [.titled, .closable, .miniaturizable, .resizable],
            backing: .buffered,
            defer: false
        )
        window.center()
        window.title = "t-read"
        window.titlebarAppearsTransparent = true
        window.isMovableByWindowBackground = true
        window.appearance = NSAppearance(named: theme == "light" ? .aqua : .darkAqua)
        window.backgroundColor = theme == "light"
            ? NSColor(calibratedWhite: 0.94, alpha: 1.0)
            : NSColor(calibratedWhite: 0.08, alpha: 1.0)

        let config = WKWebViewConfiguration()
        let controller = WKUserContentController()
        config.websiteDataStore = WKWebsiteDataStore.default()
        config.preferences.javaScriptCanOpenWindowsAutomatically = true
        let css = makeInjectedCSS(theme: theme, skinScrollbar: skinScrollbar)
        let cssJS = String(reflecting: css)
        let js = """
        (function() {
          const css = \(cssJS);
          const styleID = "tread-theme-style";
          function injectTheme() {
            const root = document.head || document.documentElement;
            if (!root) return;
            let style = document.getElementById(styleID);
            if (!style) {
              style = document.createElement("style");
              style.id = styleID;
              root.appendChild(style);
            }
            if (style.textContent !== css) {
              style.textContent = css;
            }
          }
          injectTheme();
          document.addEventListener("DOMContentLoaded", injectTheme, { once: true });
          window.addEventListener("load", injectTheme, { once: true });
          setTimeout(injectTheme, 800);
        })();
        """
        controller.addUserScript(WKUserScript(source: js, injectionTime: .atDocumentStart, forMainFrameOnly: false))
        config.userContentController = controller

        webView = WKWebView(frame: window.contentView!.bounds, configuration: config)
        webView.uiDelegate = self
        webView.autoresizingMask = [.width, .height]
        window.contentView?.addSubview(webView)
        currentZoom = loadZoom(path: zoomStatePath, minValue: minZoom, maxValue: maxZoom)
        applyZoom(currentZoom)
        installZoomShortcuts()

        if let url = URL(string: targetURL) {
            webView.load(URLRequest(url: url))
        }

        window.makeKeyAndOrderFront(nil)
        NSApp.activate(ignoringOtherApps: true)
    }

    func applicationShouldTerminateAfterLastWindowClosed(_ sender: NSApplication) -> Bool {
        return true
    }

    func applicationWillTerminate(_ notification: Notification) {
        if let monitor = zoomMonitor {
            NSEvent.removeMonitor(monitor)
        }
    }

    func clampZoom(_ value: CGFloat) -> CGFloat {
        return min(max(value, minZoom), maxZoom)
    }

    func applyZoom(_ value: CGFloat) {
        let zoom = clampZoom(value)
        currentZoom = zoom
        saveZoom(zoom, path: zoomStatePath)
        if #available(macOS 11.0, *) {
            webView.pageZoom = zoom
        } else {
            let js = "document.documentElement.style.zoom = '\(zoom)';"
            webView.evaluateJavaScript(js, completionHandler: nil)
        }
    }

    func installZoomShortcuts() {
        zoomMonitor = NSEvent.addLocalMonitorForEvents(matching: .keyDown) { [weak self] event in
            guard let self else { return event }
            guard event.modifierFlags.intersection(.deviceIndependentFlagsMask).contains(.command) else {
                return event
            }

            let key = event.charactersIgnoringModifiers ?? ""
            switch key {
            case "=", "+":
                self.applyZoom(self.currentZoom + self.zoomStep)
                return nil
            case "-", "_":
                self.applyZoom(self.currentZoom - self.zoomStep)
                return nil
            case "0":
                self.applyZoom(1.0)
                return nil
            default:
                return event
            }
        }
    }

    func webView(_ webView: WKWebView,
                 createWebViewWith configuration: WKWebViewConfiguration,
                 for navigationAction: WKNavigationAction,
                 windowFeatures: WKWindowFeatures) -> WKWebView? {
        if navigationAction.targetFrame == nil, let url = navigationAction.request.url {
            webView.load(URLRequest(url: url))
        }
        return nil
    }
}

let app = NSApplication.shared
ProcessInfo.processInfo.processName = "t-read"
let delegate = AppDelegate()
app.setActivationPolicy(.regular)
app.delegate = delegate
app.run()
