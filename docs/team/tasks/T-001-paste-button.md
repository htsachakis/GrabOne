# T-001: Paste button next to Analyze

Type: feature
Status: Done
Release: Not released
Owner: developer
Blocked by: none
Requirements: owner chat request 2026-10-20; design/approval-001.md
Branch: feat/T-001-paste-button  ·  PR: https://github.com/htsachakis/GrabOne/pull/1  ·  Head SHA: 6b96e0c

## Objective
The user can fill the link field from the clipboard with one click, without focusing the field and pressing a shortcut.

## Acceptance criteria
- [ ] AC-1 A "Paste" button sits in the URL row immediately before Analyze, using the existing secondary `.button` style; Analyze stays the primary action.
- [ ] AC-2 Clicking Paste replaces the field's contents with the clipboard text (trimmed), puts focus in the field, and enables Analyze when the text is non-empty. It does not start an analysis by itself.
- [ ] AC-3 Empty clipboard, or a clipboard read that fails: the field is left unchanged and no error is thrown to the console as uncaught.
- [ ] AC-4 While an analysis runs (field is read-only), Paste is disabled.
- [ ] AC-5 Keyboard accessible: reachable by Tab, has an accessible name and a title tooltip; existing Enter/Escape behaviour in the field is unchanged.
- [ ] AC-6 `npm run build` in frontend/ passes; `go vet ./...` and `go test ./...` still pass.

## Scope notes
Relevant components/files: frontend/src/components/UrlInput.ts, frontend/src/styles.css (only if the row needs it), frontend/src/api.ts (clipboard helper beside the other Wails calls).
Read the clipboard with the Wails runtime `ClipboardGetText()` (frontend/wailsjs/runtime/runtime.d.ts) — it works in the WebView without a browser permission prompt; `api.ts` already imports from that runtime.
Out of scope: auto-analyze on paste, clipboard watching, any other UI change.
Skills: none special (no frontend test harness exists; verify by build + running the app).
Risks / open questions: none.

## Implementation summary
- `frontend/src/api.ts`: `readClipboardText()` wraps the Wails `ClipboardGetText()`, trims, and returns "" on empty or on a rejected read (AC-3).
- `frontend/src/components/UrlInput.ts`: secondary `.button` "Paste" (title tooltip, text is its accessible name) between the field and Analyze (AC-1, AC-5). `paste()` sets the field, calls `syncButton()` and focuses the field; it does not analyze (AC-2). `update()` disables it while analyzing; `paste()` also ignores a result that arrives once the field is read-only (AC-4). Enter/Escape handlers untouched. No CSS change: `.url-row` is already a flex row with a gap.

## Verification
| Check | Result | Evidence | SHA |
|---|---|---|---|
| `cd frontend && npm run build` | passed | tsc + vite build OK | 6b96e0c |
| `go vet ./...` | passed | exit 0 | 6b96e0c |
| `go test ./...` | failed (pre-existing) | 4 failures: TestYtDlpAssetSelection (internal/tooling); 3 in internal/dependencies (aria2c found on this Mac, Windows-path update commands). Same failures on base 72210e0 via git stash (tooling: confirmed; dependencies: 3 fails confirmed). Unrelated to this change. | 6b96e0c |
| Run the app / click Paste (AC-1..5 behaviour) | not run | `wails` CLI not installed; no frontend test harness | 6b96e0c |

## Review
Approve at 6b96e0c (reviewer, 2026-10-20). Blocking: none. Suggestions (no action for T-001): field resets to state.url on re-render if user pastes then leaves without analyzing (pre-existing behaviour, same for typed text); multi-line clipboard flattened by <input> (same as Cmd+V); Wails Call has no timeout (unlikely). go test: 4 pre-existing failures on macOS, identical on base 72210e0; diff has no Go changes.

## QA
Revision tested: 6b96e0c72a8ea1a7680037d9b280392a126eda63 (branch feat/T-001-paste-button, main checkout). Environment: macOS, Vite 7.3.6 dev server (frontend/, port 5199), Chromium via Playwright MCP, viewports 1280/768/390. Wails runtime and Go bindings were STUBBED with an init script (window.runtime.ClipboardGetText controllable: text / "" / null / reject / deferred; AnalyzeURL never resolves; other App methods return empty or sensible fixtures). Screenshots: /private/tmp/claude-501/-Users-htsachakis-Developing-GrabOne/a466f37a-e015-4e53-9b87-d0fb59a8d87a/scratchpad/*.png.
Limitation: the real Wails WebView and the real `ClipboardGetText()` (OS clipboard) were NOT exercised. Everything below proves the frontend logic against the contract in runtime.d.ts, not the native clipboard.

| AC | Result | Evidence |
|---|---|---|
| AC-1 placement/style | passed | DOM order field, Paste, Analyze, Cancel(hidden). Paste uses class `button` (bg rgb(32,32,38), 13px/500); Analyze `button primary` (rgb(91,140,255)). Same height (38px), 10px gap. 01-initial-1280.png |
| AC-2 fill/trim/focus/enable | passed | clipboard "  https://example.com/watch?v=abc  " -> field "https://example.com/watch?v=abc", Analyze enabled, AnalyzeURL calls = 0. With field blurred (activeElement BODY) -> after click activeElement = url-field. Pasting over existing text replaces it; "\n\t https://vimeo.com/123 \n" trimmed. 02-after-paste-1280.png |
| AC-3 empty/failed read | passed | "" , null, whitespace-only and rejected read: field stays "keep-me"; on empty field Analyze stays disabled; window error/unhandledrejection listeners recorded 0 events; console errors 0 (only a favicon.ico 404 from Vite, unrelated). Note: focus stays on the Paste button in these cases (field not focused) |
| AC-4 disabled while analyzing | passed | Enter in field -> field readOnly, Paste `disabled`, Cancel shown; forced click -> ClipboardGetText not called (delta 0), field unchanged. Race: clipboard read pending, analysis starts, read resolves late with different text -> field unchanged (guard works). Escape cancels (CancelAnalysis called once) and re-enables Paste. 05-analyzing-1280.png |
| AC-5 keyboard/a11y | passed | Tab from field -> Paste -> Analyze (when enabled); Shift+Tab back to field; while analyzing Tab skips Paste (Cancel next). Visible focus ring (outline 2px solid rgb(91,140,255), offset 1px, :focus-visible true) 04-paste-focused-1280.png. Enter and Space on focused Paste both paste and move focus to field, no analysis. Accessible name "Paste" (ariaSnapshot: button "Paste"), title "Paste the link from the clipboard", type=button. Enter in field starts analysis with the pasted value, Escape cancels: unchanged. Native tooltip rendering not screenshot-able; title attribute verified only |
| AC-6 build/vet/test | passed (build, vet); test failed pre-existing | `npm run build`: tsc + vite OK (71.33 kB js). It deleted frontend/dist/.gitkeep; restored with `git checkout -- frontend/dist/.gitkeep`. `go vet ./...` exit 0. `go test ./...`: same 4 failures the developer reported (internal/tooling TestYtDlpAssetSelection; internal/dependencies TestAbsentOptionalExtraIsNotAnError, TestUpdateCommandFollowsWhatInstalledTheTool, TestPipUpdateUsesThePythonThatOwnsTheScriptsFolder), all environment-dependent (aria2c present on this Mac, Windows paths). The branch diff touches only frontend/src/api.ts and UrlInput.ts, no Go. Baseline run not repeated by QA |

Observations (not defects of this change):
1. Pre-existing: a link typed OR pasted is cleared when the field is blurred and the store re-renders (e.g. toggle Settings twice, then the field is empty again), because UrlInput.update() resets the input to state.url when the field is not focused. Reproduced with typed text too; the diff does not touch that path. Paste makes it equally reachable. Low severity; suggest a follow-up task, not a blocker.
2. At 390px wide, while analyzing (Cancel visible) the URL field shrinks to 40px (was roughly 104px before Paste was added). The real window has MinWidth 880 (main.go), so this is not reachable in the app. 768px is fine (field 418px). 06-analyzing-390.png
3. Test-harness caveat: Vite serves the unstubbed app; no automated test was added (no frontend test harness exists; no product code changed).

Not tested: real WebView/Wails clipboard (permissions, non-text content, large text, Windows/macOS differences), packaged build, Windows. Verdict: Ready (frontend behaviour); recommend a 1-minute manual smoke in the real app (`wails dev`) with a real clipboard before merge.

## Log
### 2026-10-20 developer — Paste button implemented, PR opened
Revision: 6b96e0c72a8ea1a7680037d9b280392a126eda63
Completed: AC-1..AC-5 in code; AC-6 build and vet pass; go test has pre-existing platform failures.
Changed: frontend/src/api.ts, frontend/src/components/UrlInput.ts (docs/team not committed)
Verified: see Verification table
Uncertain: behaviour in the real WebView not exercised (no wails CLI); README has no mention of the URL row so no docs change.
Blocked: none
Questions for the owner: none
Next: reviewer on 6b96e0c, then QA to run the app and click through AC-2..AC-5

### 2026-10-20 qa-engineer — AC-1..AC-6 verified against stubbed Wails runtime; Ready, one real-clipboard smoke recommended
Revision: 6b96e0c72a8ea1a7680037d9b280392a126eda63
Completed: behaviour tested in Chromium (Playwright) on Vite dev server with stubbed window.runtime / window.go; build, vet, go test run.
Changed: this task file only (QA section, Log). No product code, no commits. frontend/dist/.gitkeep restored after build.
Verified: AC-1 passed; AC-2 passed; AC-3 passed; AC-4 passed (incl. late-resolve race); AC-5 passed; AC-6 build passed, go vet passed, go test failed with the 4 pre-existing environment failures (evidence: QA section; screenshots in /private/tmp/claude-501/-Users-htsachakis-Developing-GrabOne/a466f37a-e015-4e53-9b87-d0fb59a8d87a/scratchpad)
Uncertain: real Wails WebView / OS clipboard NOT exercised (stubbed) — the one thing this run cannot prove. Native tooltip display not seen, only the title attribute.
Blocked: none (wails CLI absent, handled by stubbing)
Questions for the owner: none. Recommendation: run `wails dev` once and click Paste with real clipboard text before merge.
Next: tech-lead — merge decision after reviewer; consider follow-up task for the pre-existing field-reset-on-blur behaviour (observation 1).

## Next action
reviewer reviews PR #1 at 6b96e0c; QA runs the app

### 2026-10-20 tech-lead — merged
Revision: 902107e (squash of 6b96e0c, PR #1) after owner's yes. CI Go+Frontend passed at 6b96e0c.
