# T-003: Keep an unanalyzed link when the window re-renders

Type: bug
Status: Done
Release: Not released
Owner: developer (Claude session local_0ae482d1)
Blocked by: none
Requirements: found by review/QA of T-001
Branch: fix/keep-unsubmitted-link  ·  PR: https://github.com/htsachakis/GrabOne/pull/3  ·  Head SHA: 17eebae

## Objective
A link typed or pasted into the field but not yet analyzed survives re-renders (e.g. toggling Settings twice).

## Acceptance criteria
- [ ] AC-1 Typed or Paste-button text stays in the field after focus leaves and the store re-renders.
- [ ] AC-2 The field still follows `state.url` when the URL is set elsewhere (e.g. starting another download of the same media).
- [ ] AC-3 Paste button behaviour from T-001 (approval-001) unchanged: fills trimmed text, focuses, no auto-analyze, disabled while analyzing.
- [ ] AC-4 `npm run build` passes; CI green.

## Review
Approve at 17eebae (reviewer, 2026-10-20); holds for merged 8b42b83 (tree identical to merge-tree of 902107e+17eebae). Blocking: none. Suggestions: comment at UrlInput.ts:112-113 names a path that doesn't set state.url (should say 'a new analysis or Download another'); shownUrl advances while field focused (no current path hits it). tsc + vite build on 8b42b83 passed; no CI run on 8b42b83 (CI runs on PRs only).
Note: PR merged by the follow-up Claude session (via owner's gh account) at 2026-10-20T07:00:57Z, before team review/QA finished; review done afterwards.

## QA
Revision tested: INTEGRATED = 902107e (main with Paste button, T-001) + 17eebaedb375b30d979df03cf388bb045dcef86b (PR #3 head), `git merge --no-ff --no-commit` in a throwaway worktree, nothing committed. Resulting tree d6877ddc04e94cd03c7c6591c3afe37d57c1c7be, identical to `origin/main^{tree}` (8b42b83, the squash merge of PR #3 that landed 2026-10-20T07:00:57Z while QA was running). So the tested tree is exactly what is on main now. Merge was automatic and touched only frontend/src/components/UrlInput.ts (api.ts from 17eebae was a no-op on top of 902107e: `readClipboardText` kept, see Observation 2).
Environment: macOS, Vite dev server (port 5201 plain 902107e for the bug repro, 5202 integrated), Chromium via Playwright MCP, 1280x800. Wails runtime and Go bindings STUBBED with an init script (`/private/tmp/claude-501/-Users-htsachakis-Developing-GrabOne/a466f37a-e015-4e53-9b87-d0fb59a8d87a/scratchpad/stub.js`): window.runtime.ClipboardGetText controllable (text / "" / null / reject / deferred), EventsOnMultiple records handlers so backend events can be emitted by hand, window.go.main.App.AnalyzeURL either never resolves (holds "analyzing") or resolves a fake media, StartDownload returns a fake job, the rest return empty/fixtures.
Limitation: the real Wails WebView and the real OS clipboard were NOT exercised. This proves the frontend logic against the runtime.d.ts contract only.

| Check | Result | Evidence |
|---|---|---|
| Bug reproduced on plain 902107e (so the fix matters) | passed (bug confirmed) | type link, click away (activeElement BODY), toggle Settings twice -> field "" (was typed text). Same after Paste-button text: field had "https://example.com/pasted", blur + toggle x2 -> "". t3-base-bug.png |
| AC-1 typed text survives blur + re-render (integrated) | passed | typed "https://example.com/watch?v=typed", blur, Settings toggled twice -> value kept, Analyze enabled; also kept after an emitted `dependencies:changed` store update. t3-integ-typed-after-toggle.png |
| AC-1 Paste-button text survives (integrated) | passed | clipboard "  https://example.com/pasted  " -> field trimmed, blur, toggle x2 -> kept. Editing it to another value then blur + toggle -> new value kept. t3-integ-pasted-after-toggle.png |
| AC-2 field follows state.url when set elsewhere | passed | Only two code paths set `url` (grep app.ts): `analyze(url)` and `resetForNext()` (the "Download another" button). Exercised with stubs: analyze A (Enter) -> ready, field A. Typed unsubmitted B, blur, toggle -> B kept. Download clicked (StartDownload called with A), emitted `download:state` completed -> "Download another" shown, field still B (state.url unchanged, correct). Clicked "Download another" -> state.url "" -> field cleared to "" and focused (id url-field). Then typed C, blur, toggle -> C kept. Then analyzed D via Enter, blur, toggle -> D. t3-integ-completed.png, t3-integ-after-another.png |
| AC-3 Paste: trimmed fill | passed | clipboard "\n\t https://vimeo.com/123 \n" -> "https://vimeo.com/123" |
| AC-3 Paste: focus, no auto-analyze, Analyze enabled | passed | activeElement id `url-field` after click (clicked from blurred state); AnalyzeURL calls 0; Analyze enabled |
| AC-3 Paste: disabled while analyzing | passed | Enter -> field readOnly, Paste disabled, Cancel visible; forced `.click()` on Paste -> ClipboardGetText calls +0, field unchanged. Escape -> CancelAnalysis x1, Paste re-enabled, field text kept, and kept again after blur + toggle x2. Late-resolving clipboard read after analysis started -> field unchanged ("race" guard intact). t3-integ-analyzing.png |
| AC-3 Paste: empty / rejected clipboard | passed | "", null, whitespace-only, rejected read: field stays "keep-me" each time; empty field keeps Analyze disabled; window error/unhandledrejection listeners recorded 0 events |
| AC-3 DOM order/keyboard | passed | row order field, Paste (`button`), Analyze (`button primary`), Cancel (hidden). Tab from field -> Paste; Enter on Paste fills and focuses the field |
| AC-4 `npm run build` | passed | tsc + vite build OK on the integrated tree, 71.36 kB js, exit 0. It deleted frontend/dist/.gitkeep in the throwaway worktree only; the real checkout's file is untouched (`git ls-files frontend/dist` lists it, status clean) |
| AC-4 CI green | passed | `gh pr checks 3`: Frontend pass (38s), Go pass (1m47s) |

Observations (not defects):
1. Behaviour change worth knowing, by design of the fix: the field now re-syncs only when `state.url` changes. If the field is focused at the moment `state.url` changes, `shownUrl` is updated but the field is not, so it keeps what the user typed. In the current app this cannot happen (analyze takes the value from the field; "Download another" moves focus to the button first), so not reachable; would matter only if a future feature sets `url` while the user is typing.
2. PR #3 branched from 72210e0 (before T-001), so 17eebae alone shows `readClipboardText` removed from api.ts; on top of 902107e the merge keeps it (git auto-merge, final tree verified to contain the Paste button and helper). Squash 8b42b83 on main has the same tree, so no regression on main.
3. Not tested: real Wails WebView/clipboard, packaged build, Windows, `go test` (PR touches no Go; CI Go job passed), 390/768 px layout (no CSS change).

Verdict: Ready (frontend logic, integrated 902107e + 17eebae). Already merged as 8b42b83; recommend the same 1-minute `wails dev` smoke with the real clipboard noted in T-001.

## Log

### 2026-10-20 qa-engineer — AC-1..AC-4 verified on integrated 902107e + 17eebae; Ready
Revision: 902107e + 17eebaedb375b30d979df03cf388bb045dcef86b merged in a throwaway worktree (tree d6877dd = origin/main 8b42b83, PR #3 already squash-merged during the run)
Completed: reproduced the bug on plain 902107e, then AC-1..AC-3 on the integrated tree in Chromium (Playwright) against stubbed Wails runtime; build and CI checked.
Changed: this task file only (QA section, Log). No product code, no commits or pushes. Worktrees removed, Vite servers stopped. Screenshots: /private/tmp/claude-501/-Users-htsachakis-Developing-GrabOne/a466f37a-e015-4e53-9b87-d0fb59a8d87a/scratchpad/t3-*.png; stub: /private/tmp/claude-501/-Users-htsachakis-Developing-GrabOne/a466f37a-e015-4e53-9b87-d0fb59a8d87a/scratchpad/stub.js
Verified: bug repro on base — passed (field emptied); AC-1 — passed; AC-2 — passed (via analyze + Download another path); AC-3 — passed; AC-4 build — passed; CI Frontend+Go — passed (evidence: QA section)
Uncertain: real Wails WebView / OS clipboard NOT exercised (stubbed).
Blocked: none
Questions for the owner: none. Recommendation: one `wails dev` smoke with a real clipboard.
Next: tech-lead — mark T-003 Done (already merged as 8b42b83).
