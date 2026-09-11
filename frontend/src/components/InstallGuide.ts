import { el } from "../dom";
import type { InstallGuidance } from "../types";

export interface InstallGuideHandlers {
  onOpenURL: (url: string) => void;
}

/**
 * installGuide renders how to install a dependency GrabOne could not find.
 *
 * GrabOne never installs these tools itself: they are separate programs with
 * their own release cycles. What it can do is say exactly how to get them, with
 * a command ready to copy and a link to the download page.
 */
export function installGuide(guidance: InstallGuidance, handlers: InstallGuideHandlers): HTMLElement {
  const steps = guidance.steps.map((step, index) =>
    el("li", { className: "install-step" }, [
      el("div", { className: "install-step-head" }, [
        el("span", { className: "install-step-number", text: String(index + 1) }),
        el("span", { className: "install-step-title", text: step.title }),
      ]),
      step.command
        ? el("div", { className: "install-command-row" }, [
            el("code", { className: "install-command", text: step.command }),
            copyButton(step.command),
          ])
        : null,
      step.url
        ? el("button", {
            className: "button subtle small",
            text: "Open download page",
            attrs: { type: "button" },
            on: { click: () => handlers.onOpenURL(step.url as string) },
          })
        : null,
      el("p", { className: "hint", text: step.detail }),
    ]),
  );

  return el("details", { className: "install-guide" }, [
    el("summary", { text: `How to install ${guidance.displayName}` }),
    el("p", { className: "hint", text: guidance.summary }),
    el("ol", { className: "install-steps" }, steps),
    el("p", { className: "hint", text: guidance.afterInstall }),
  ]);
}

/** copyButton copies a command to the clipboard and confirms it briefly. */
function copyButton(command: string): HTMLElement {
  let timer = 0;

  const button = el("button", {
    className: "button subtle small",
    text: "Copy",
    attrs: { type: "button" },
    on: {
      click: () => {
        void navigator.clipboard
          .writeText(command)
          .then(() => {
            button.textContent = "Copied";
          })
          .catch(() => {
            button.textContent = "Copy failed";
          })
          .finally(() => {
            window.clearTimeout(timer);
            timer = window.setTimeout(() => {
              button.textContent = "Copy";
            }, 1800);
          });
      },
    },
  });
  return button;
}
