// The wits window: the interface wits-server serves, in a window of its own
// rather than a browser tab. The AppImage's AppRun starts the server and
// opens this on it (WITS_URL); closing the window stops them both. Nothing of
// Node reaches the page — it is the same page a browser gets — and anything
// it links to off this server opens in the system's browser instead.
//
//   WITS_URL     the server to show (default http://127.0.0.1:7341)
//   WITS_ICON    the window's icon, a PNG
//   WITS_SYSTEM_LIBRARY_PATH
//                the system's LD_LIBRARY_PATH, before AppRun put the image's
//                own libraries first: a browser a link opens gets it back
//   WITS_SMOKE   set, and the window loads the page, prints its title and
//                quits: what appimage.sh check runs
import { spawn } from 'node:child_process';

import { app, BrowserWindow, shell } from 'electron';

const url = process.env.WITS_URL || 'http://127.0.0.1:7341';
const origin = new URL(url).origin;

/** Whether a link stays on the server, as the interface's own pages do. */
function own(link) {
  try {
    return new URL(link).origin === origin;
  } catch {
    return false;
  }
}

/**
 * Opens a link off the server in the system's browser; anything else is
 * refused. From the AppImage, the browser is started with the system's own
 * libraries, not the NSS this window carries.
 */
function away(link) {
  if (!/^https?:/.test(link)) {
    return;
  }
  if (process.platform !== 'linux' || !('WITS_SYSTEM_LIBRARY_PATH' in process.env)) {
    void shell.openExternal(link);
    return;
  }
  // The window's own settings stay with it, and the image's libraries with them.
  const own = new Set(['WITS_SYSTEM_LIBRARY_PATH', 'WITS_URL', 'WITS_ICON', 'WITS_SMOKE', 'LD_LIBRARY_PATH']);
  const env = Object.fromEntries(Object.entries(process.env).filter(([key]) => !own.has(key)));
  if (process.env.WITS_SYSTEM_LIBRARY_PATH) {
    env.LD_LIBRARY_PATH = process.env.WITS_SYSTEM_LIBRARY_PATH;
  }
  const browser = spawn('xdg-open', [link], { env, detached: true, stdio: 'ignore' });
  browser.on('error', () => void shell.openExternal(link));
  browser.unref();
}

let win = null;

function open() {
  win = new BrowserWindow({
    width: 1440,
    height: 960,
    minWidth: 375,
    minHeight: 560,
    title: 'wits',
    icon: process.env.WITS_ICON || undefined,
    backgroundColor: '#0b0f17',
    autoHideMenuBar: true,
    show: !process.env.WITS_SMOKE,
    webPreferences: { contextIsolation: true, sandbox: true, nodeIntegration: false },
  });
  // A new window is never opened here: the terpene sources, the searches and
  // the like go to the browser.
  win.webContents.setWindowOpenHandler(({ url: link }) => {
    away(link);
    return { action: 'deny' };
  });
  win.webContents.on('will-navigate', (event, link) => {
    if (!own(link)) {
      event.preventDefault();
      away(link);
    }
  });
  if (process.env.WITS_SMOKE) {
    win.webContents.once('did-finish-load', () => {
      console.log(`wits-desktop: loaded ${win.webContents.getTitle()}`);
      app.quit();
    });
    win.webContents.once('did-fail-load', (_e, code, why) => {
      console.error(`wits-desktop: ${url} did not load: ${why} (${code})`);
      app.exit(1);
    });
  }
  void win.loadURL(url);
  win.on('closed', () => {
    win = null;
  });
}

// One window: launched again, the one open comes to the front.
if (!app.requestSingleInstanceLock()) {
  app.quit();
} else {
  app.on('second-instance', () => {
    if (win) {
      if (win.isMinimized()) {
        win.restore();
      }
      win.focus();
    }
  });
  app.whenReady().then(open);
  app.on('window-all-closed', () => app.quit());
}
