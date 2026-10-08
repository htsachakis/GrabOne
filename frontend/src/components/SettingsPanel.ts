import { el, replace } from "../dom";
import type { AppState } from "../state";
import type {
  DependencyStatus as DependencyState,
  InstallGuidance,
  Settings,
  ToolProgress,
  ToolSource,
  UpdateStatus,
} from "../types";
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
 * SettingsPanel groups the dependency locations, the download defaults, the
 * speed settings and the authentication choice.
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

    // The aria2c switch depends on whether aria2c was found, so its status is
    // part of what the sections are drawn from.
    const aria2c = state.dependencies?.aria2c ?? null;
    const signature =
      JSON.stringify(settings) +
      String(state.appInfo?.version) +
      JSON.stringify(this.updateStatus) +
      JSON.stringify(aria2c);
    if (signature === this.signature) return;
    this.signature = signature;

    replace(this.sections, [
      this.downloadsSection(settings),
      this.speedSection(settings, aria2c),
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

  /**
   * speedSection holds the speed settings: the choices that change how fast a
   * job fetches its media, and never what is saved.
   *
   * They apply to jobs started from now on. A job that fails with any of them
   * active is tried once more without them, so a setting a site does not accept
   * costs time and not the download.
   */
  private speedSection(settings: Settings, aria2c: DependencyState | null): HTMLElement {
    const counts = Array.from({ length: 16 }, (_, index) => index + 1);
    const aria2cFound = aria2c?.available === true;

    return el("section", { className: "panel" }, [
      el("h2", { className: "panel-title", text: "Speed" }),
      el("p", {
        className: "hint",
        text: "For sites that slow each connection down. These apply to downloads you start from now on. If a download fails with them, GrabOne tries it once more without them.",
      }),

      el("div", { className: "field" }, [
        el("span", { className: "field-label", text: "Connections per download" }),
        el(
          "select",
          {
            className: "select narrow",
            on: {
              change: (event) =>
                this.save(settings, {
                  connections: Number((event.currentTarget as HTMLSelectElement).value),
                }),
            },
          },
          counts.map((count) =>
            el("option", {
              text: String(count),
              attrs: { value: count, selected: count === settings.connections },
            }),
          ),
        ),
        el("p", {
          className: "hint",
          text:
            settings.useAria2c && aria2cFound
              ? "How many pieces of one download are fetched at the same time. Media a site delivers in pieces is fetched this way by yt-dlp, and a single-file stream by aria2c."
              : "How many pieces of one download are fetched at the same time. It only helps media a site delivers in pieces; a single-file stream still uses one connection unless aria2c is switched on below.",
        }),
        settings.connections > 4
          ? el("p", {
              className: "hint warning",
              text: "High values can get you rate limited or blocked by a site, more so with several simultaneous downloads.",
            })
          : el("span"),
      ]),

      el("div", { className: "field" }, [
        this.checkbox("Chunked transfer", settings.chunkedTransfer, (checked) =>
          this.save(settings, { chunkedTransfer: checked }),
        ),
        el("p", {
          className: "hint",
          text: "Asks for a single-file stream in 10 MB pieces, which gets past some sites that slow a long transfer down. It makes other sites slower, so switch it on only where it helps.",
        }),
      ]),

      this.aria2cField(settings, aria2cFound, aria2c?.version ?? ""),
    ]);
  }

  /**
   * aria2cField is the switch that hands single-file streams to aria2c.
   *
   * It can only be switched on once aria2c is found, and the offer to download
   * it sits beside it until then. A switch that is already on stays usable when
   * aria2c goes missing, so it can always be switched off again.
   */
  private aria2cField(settings: Settings, found: boolean, version: string): HTMLElement {
    const children: (HTMLElement | false)[] = [
      this.checkbox(
        "Use aria2c for single-file streams",
        settings.useAria2c,
        (checked) => this.save(settings, { useAria2c: checked }),
        !found && !settings.useAria2c,
      ),
    ];

    if (found) {
      children.push(
        el("p", {
          className: "hint",
          text: `Fetches a single-file stream over as many connections as set above, using aria2c ${version}. It takes over from chunked transfer for those streams, and its progress updates once a second.`,
        }),
        settings.useAria2c &&
          settings.connections < 2 &&
          el("p", {
            className: "hint warning",
            text: "With one connection aria2c has nothing to add, so it is not used. Raise the connections to use it.",
          }),
      );
    } else {
      children.push(
        el("p", {
          className: settings.useAria2c ? "hint warning" : "hint",
          text: settings.useAria2c
            ? "aria2c was not found, so downloads run without it until it is installed."
            : "aria2c is a separate, optional program that is not installed. It fetches a single-file stream over several connections, which yt-dlp cannot do itself.",
        }),
        el("div", { className: "download-actions" }, [
          el("button", {
            className: "button subtle small",
            text: "Download aria2c",
            title: "Downloads aria2c from its official release and verifies it against the checksum built into GrabOne",
            attrs: { type: "button" },
            on: { click: () => this.handlers.onInstallTool("aria2c") },
          }),
        ]),
      );
    }
    return el("div", { className: "field" }, children);
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

  private checkbox(
    label: string,
    checked: boolean,
    onToggle: (value: boolean) => void,
    disabled = false,
  ): HTMLElement {
    const input = el("input", {
      attrs: { type: "checkbox", checked: checked ? "" : null, disabled: disabled ? "" : null },
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
