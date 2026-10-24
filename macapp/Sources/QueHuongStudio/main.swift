import AppKit

Lang.refresh() // before Sparkle loads, so its dialogs use the chosen language
let app = NSApplication.shared
let delegate = AppDelegate()
app.delegate = delegate
app.setActivationPolicy(.regular)
app.run()
