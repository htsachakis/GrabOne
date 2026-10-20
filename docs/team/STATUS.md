# GrabOne — team status

Updated: 2026-10-20 by tech-lead   ·   Stage: 12 Maintenance (released v1.1.6)   ·   Revision: d789988

## Kickoff
- App type: desktop app (Wails: Go backend + TypeScript/Vite frontend, no framework), Windows installer via NSIS
- Hosting: GitHub htsachakis/GrabOne
- Deployment target: GitHub release produced by a version tag (docs/releasing.md)
- Stack: Go, Wails v2, TypeScript + Vite (plain DOM helpers in frontend/src/dom.ts), yt-dlp as external tool
- Base branch and conventions: main; commit subjects are plain imperative sentences ("Run yt-dlp in the temporary folder"); domain language in CONTEXT.md

## Authority
Default policy: `~/.claude/skills/dev-team/reference/authority.md` (team goes up to PR; owner approves merge and every release tag).
Overrides for this project:
- none

## Where things live
Defaults from `reference/tracking.md`. Exceptions: architecture notes in docs/architecture.md, release process in docs/releasing.md, glossary in CONTEXT.md (existing).

## Tasks
| ID | Title | Owner | Status | Release | Next action |
|---|---|---|---|---|---|
| T-001 | Paste button next to Analyze | developer | Done | Released v1.1.6 | merged 902107e (squash, PR #1) |
| T-002 | Go tests pass on macOS | developer (session local_6d3ac806) | Done | Released v1.1.6 | merged a1d8334 (PR #2) |
| T-003 | Keep unanalyzed link on re-render | developer (session local_0ae482d1) | Done | Released v1.1.6 | merged 8b42b83; review approve; QA ready (tree identical to main) |

## Dependencies
none

## Needs the owner
- none

## Decisions log
- 2026-10-20 owner chose v1.1.6 (over recommended v1.2.0) and asked to release; tag v1.1.6 pushed at d789988
- 2026-10-20 PR #3 (8b42b83) and PR #2 (a1d8334) were merged by the follow-up Claude sessions themselves (via the owner's gh account), before the owner's yes; owner then said "merge all". Correction of earlier note attributing #3 to the owner. Future follow-up session briefs must say: open the PR, do not merge.
- 2026-10-20 owner: merge PR #1 → squash-merged as 902107e
- 2026-10-20 owner started follow-ups T-002 (PR #2) and T-003 (PR #3)
- 2026-10-20 Paste button design taken from the owner's chat request; recorded as design/approval-001.md

## Blockers
- none

## Release state
- Current version: v1.1.6 (tag at d789988, release run 39386017785 success; installer, portable exe and checksums.txt published, marked Latest) · Real-app Paste smoke with live clipboard: not run

## Next actions
2. owner: one `wails dev` smoke with real clipboard (Paste + link kept on re-render)
