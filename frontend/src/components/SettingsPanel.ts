import { el, replace } from "../dom";
import type { AppState } from "../state";
import type { InstallGuidance, Settings, ToolProgress, ToolSource, UpdateStatus } from "../types";
import { DependencyStatus } from "./DependencyStatus";

export interface SettingsHandlers {
  onSave: (settings: Settings) => void;
  onLocate: (name: string) => void;
  onRetry: () => void;
  onBrowseOutput: () => void;
  onBrowseCookieFile: () => void;
  onOpenLogs: () => void;
  onOpenURL: (url: string) => void;
  onInstallTool: (name: string) => void;
  onUpdateTool: (name: string) => void;
  onCheckUpdate: () => void;
  onDownloadUpdate: () => void;
  onInstallUpdate: () => void;
  onClose: () => void;
}

/**
 * SettingsPanel groups the dependency locations, the download defaults and the
 * authentication choice.
 *
 * Authentication is handled by handing yt-dlp a browser name or a cookie file.
 * GrabOne never reads a browser's cookie store itself and never logs cookie
 * contents.
 */
export class SettingsPanel {
  readonly element: HTMLElement;

  private readonly dependencies: DependencyStatus;
  private readonly sections: HTMLElement;
  private signature = "";
  private updateStatus: UpdateStatus | null = null;

  constructor(
    private readonly handlers: SettingsHandlers,
    private readonly cookieBrowsers: string[],
    installGuidance: InstallGuidance[] = [],
    toolSources: ToolSource[] = [],
  ) {
    this.dependencies = new DependencyStatus(
      {
        onLocate: handlers.onLocate,
        onRetry: handlers.onRetry,
        onOpenURL: handlers.onOpenURL,
        onInstallTool: handlers.onInstallTool,
        onUpdateTool: handlers.onUpdateTool,
      },
      true,
    );
    this.dependencies.setGuidance(installGuidance);
    this.dependencies.setToolSources(toolSources);
    this.sections = el("div", { className: "settings-sections" });

    this.element = el("div", { className: "settings-screen" }, [
      el("div", { className: "settings-header" }, [
        el("h1", { className: "screen-title", text: "Settings" }),
        el("button", {
          className: "button subtle",
          text: "Back",
          attrs: { type: "button" },
          on: { click: () => this.handlers.onClose() },
        }),
      ]),
      this.dependencies.element,
      this.sections,
    ]);
  }

  /** setToolProgress passes a running tool download to the dependency list. */
  setToolProgress(progress: ToolProgress | null): void {
    this.dependencies.setToolProgress(progress);
  }

  /** setToolError shows why a tool download did not work. */
  setToolError(name: string, message: string): void {
    this.dependencies.setToolError(name, message);
  }

  /** setUpdateStatus re-renders the updates section when a check finishes. */
  setUpdateStatus(status: UpdateStatus | null): void {
    this.updateStatus = status;
    this.signature = "";
  }

  update(state: AppState): void {
    this.dependencies.update(state.dependencies);

    const settings = state.settings;
    if (!settings) return;

    const signature = JSON.stringify(settings) + String(state.appInfo?.version) + JSON.stringify(this.updateStatus);
    if (signature === this.signature) return;
    this.signature = signature;

    replace(this.sections, [
      this.downloadsSection(settings),
      this.authenticationSection(settings),
      this.appearanceSection(settings),
      this.updatesSection(settings),
      this.aboutSection(state),
    ]);
  }

  private downloadsSection(settings: Settings): HTMLElement {
    return el("section", { className: "panel" }, [
      el("h2", { className: "panel-title", text: "Downloads" }),

      el("div", { className: "field" }, [
        el("span", { className: "field-label", text: "Default output folder" }),
        el("div", { className: "path-row" }, [
          el("input", {
            className: "path-field",
            attrs: { type: "text", readonly: "", value: settings.outputDirectory },
            title: settings.outputDirectory,
          }),
          el("button", {
            className: "button subtle",
            text: "Browse",
            attrs: { type: "button" },
            on: { click: () => this.handlers.onBrowseOutput() },
          }),
        ]),
      ]),

      el("div", { className: "field" }, [
        el("span", { className: "field-label", text: "Filename template" }),
        el("input", {
          className: "text-field",
          attrs: { type: "text", value: settings.filenameTemplate, spellcheck: "false" },
          on: {
            change: (event) =>
              this.save(settings, {
                filenameTemplate: (event.currentTarget as HTMLInputElement).value.trim(),
              }),
          },
        }),
        el("p", {
          className: "hint",
          text: "yt-dlp output template. Filename sanitisation is handled by yt-dlp itself.",
        }),
      ]),

      el("div", { className: "field" }, [
        el("span", { className: "field-label", text: "Simultaneous downloads" }),
        el(
          "select",
          {
            className: "select narrow",
            on: {
              change: (event) =>
                this.save(settings, {
                  maxConcurrentDownloads: Number((event.currentTarget as HTMLSelectElement).value),
                }),
            },
          },
          [1, 2, 3, 4, 5].map((count) =>
            el("option", {
              text: String(count),
              attrs: { value: count, selected: count === settings.maxConcurrentDownloads },
            }),
          ),
        ),
        el("p", { className: "hint", text: "Extra downloads wait in the queue until a slot is free." }),
      ]),
    ]);
  }

  private authenticationSection(settings: Settings): HTMLElement {
    const sources: { value: string; label: string }[] = [
      { value: "none", label: "None" },
      { value: "browser", label: "Cookies from a browser" },
      { value: "file", label: "cookies.txt file" },
    ];

    return el("section", { className: "panel" }, [
      el("h2", { className: "panel-title", text: "Authentication" }),
      el("p", {
        className: "hint",
        text: "Needed for private, age restricted and login-only media. Cookies are passed to yt-dlp and never stored or logged by GrabOne.",
      }),
      el(
        "div",
        { className: "radio-row" },
        sources.map((source) =>
          this.radio("cookie-source", source.label, settings.cookieSource === source.value, () =>
            this.save(settings, { cookieSource: source.value }),
          ),
        ),
      ),

      settings.cookieSource === "browser"
        ? el("div", { className: "field" }, [
            el("span", { className: "field-label", text: "Browser" }),
            el(
              "select",
              {
                className: "select narrow",
                on: {
                  change: (event) =>
                    this.save(settings, { cookieBrowser: (event.currentTarget as HTMLSelectElement).value }),
                },
              },
              [
                el("option", { text: "Select a browser", attrs: { value: "" } }),
                ...this.cookieBrowsers.map((browser) =>
                  el("option", {
                    text: browser.charAt(0).toUpperCase() + browser.slice(1),
                    attrs: { value: browser, selected: browser === settings.cookieBrowser },
                  }),
                ),
              ],
            ),
            el("p", {
              className: settings.cookieBrowser ? "hint" : "hint warning",
              text: settings.cookieBrowser
                ? "Close the browser first if it locks its cookie database."
                : "Pick a browser. Until then GrabOne downloads without cookies.",
            }),
          ])
        : el("span"),

      settings.cookieSource === "file"
        ? el("div", { className: "field" }, [
            el("span", { className: "field-label", text: "Cookie file" }),
            el("div", { className: "path-row" }, [
              el("input", {
                className: "path-field",
                attrs: { type: "text", readonly: "", value: settings.cookieFile },
                title: settings.cookieFile,
              }),
              el("button", {
                className: "button subtle",
                text: "Browse",
                attrs: { type: "button" },
                on: { click: () => this.handlers.onBrowseCookieFile() },
              }),
            ]),
            !settings.cookieFile
              ? el("p", {
                  className: "hint warning",
                  text: "Choose a cookies.txt file. Until then GrabOne downloads without cookies.",
                })
              : el("span"),
          ])
        : el("span"),
    ]);
  }

  private appearanceSection(settings: Settings): HTMLElement {
    return el("section", { className: "panel" }, [
      el("h2", { className: "panel-title", text: "Appearance" }),
      el("div", { className: "field" }, [
        el("span", { className: "field-label", text: "Theme" }),
        el("div", { className: "radio-row" }, [
          this.radio("theme", "Dark", settings.theme === "dark", () => this.save(settings, { theme: "dark" })),
          this.radio("theme", "Light", settings.theme === "light", () => this.save(settings, { theme: "light" })),
        ]),
      ]),
    ]);
  }

  /**
   * updatesSection reports what is installed and what is published.
   *
   * Checking is a request to GitHub, so it is something the user can turn off.
   * Installing always takes a further, explicit action.
   */
  private updatesSection(settings: Settings): HTMLElement {
    const status = this.updateStatus;

    const state: (HTMLElement | false | null)[] = [];
    if (status?.error) {
      state.push(el("p", { className: "hint warning", text: status.error }));
    } else if (status?.available && !status.skipped) {
      state.push(el("p", { className: "hint", text: `GrabOne ${status.latestVersion} is available.` }));
    } else if (status?.available && status.skipped) {
      state.push(el("p", { className: "hint", text: `Version ${status.latestVersion} is available but was skipped.` }));
    } else if (status?.checked) {
      state.push(el("p", { className: "hint", text: "GrabOne is up to date." }));
    }

    const actions: (HTMLElement | false)[] = [
      el("button", {
        className: "button subtle small",
        text: "Check now",
        attrs: { type: "button", disabled: status?.downloading ? "" : null },
        on: { click: () => this.handlers.onCheckUpdate() },
      }),
    ];
    if (status?.available && !status.downloadedPath && !status.downloading) {
      actions.push(
        el("button", {
          className: "button subtle small",
          text: "Download update",
          attrs: { type: "button" },
          on: { click: () => this.handlers.onDownloadUpdate() },
        }),
      );
    }
    if (status?.downloadedPath) {
      actions.push(
        el("button", {
          className: "button primary small",
          text: "Install and restart",
          attrs: { type: "button" },
          on: { click: () => this.handlers.onInstallUpdate() },
        }),
      );
    }

    return el("section", { className: "panel" }, [
      el("h2", { className: "panel-title", text: "Updates" }),
      el("div", { className: "field" }, [
        this.checkbox("Check for updates on startup", settings.autoCheckUpdates, (checked) =>
          this.save(settings, { autoCheckUpdates: checked }),
        ),
        el("p", {
          className: "hint",
          text: "Asks GitHub for the latest release. An update is downloaded and installed only when you ask for it, and its checksum is verified first.",
        }),
        ...state,
        el("div", { className: "download-actions" }, actions),
      ]),
    ]);
  }

  private checkbox(label: string, checked: boolean, onToggle: (value: boolean) => void): HTMLElement {
    const input = el("input", {
      attrs: { type: "checkbox", checked: checked ? "" : null },
      on: { change: (event) => onToggle((event.currentTarget as HTMLInputElement).checked) },
    });
    return el("label", { className: "checkbox" }, [input, el("span", { text: label })]);
  }

  private aboutSection(state: AppState): HTMLElement {
    const info = state.appInfo;
    return el("section", { className: "panel" }, [
      el("h2", { className: "panel-title", text: "About" }),
      el("dl", { className: "detail-grid" }, [
        el("dt", { text: "Version" }),
        el("dd", { text: info?.version ?? "—" }),
        el("dt", { text: "Settings file" }),
        el("dd", { text: info?.settingsPath ?? "—" }),
        el("dt", { text: "Logs" }),
        el("dd", { text: info?.logDirectory ?? "—" }),
      ]),
      el("div", { className: "download-actions" }, [
        el("button", {
          className: "button subtle small",
          text: "Open log folder",
          attrs: { type: "button" },
          on: { click: () => this.handlers.onOpenLogs() },
        }),
      ]),
      el("p", { className: "legal-note", text: info?.legalNotice ?? "" }),
    ]);
  }

  private save(settings: Settings, patch: Partial<Settings>): void {
    this.handlers.onSave({ ...settings, ...patch });
  }

  private radio(name: string, label: string, checked: boolean, onSelect: () => void): HTMLElement {
    const input = el("input", {
      attrs: { type: "radio", name, checked: checked ? "" : null },
      on: { change: () => onSelect() },
    });
    return el("label", { className: "radio" }, [input, el("span", { text: label })]);
  }
}
