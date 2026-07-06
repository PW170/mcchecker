# MCChecker

## Features

- **Cookie checking:** Paste all your cookie files into the `cookies/` folder created by opening the `.exe` file
- **MC checking:** Checks if these accounts are Minecraft-valid
- **Ban checks:** Hypixel ban checks to see if the accounts are banned or unbanned

## Requirements

- GoLang installed
- Python installed
- WebView2 Runtime (included with Windows 11, or download from Microsoft)

## Build

```bash
# GUI version (default)
wails build

# Terminal version
go build -tags terminal
```
