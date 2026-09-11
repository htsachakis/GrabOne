# Releasing GrabOne

A release is produced by pushing a version tag. Everything else — building,
checksums, the GitHub release and the notes — happens in
[`.github/workflows/release.yml`](../.github/workflows/release.yml).

## Publishing a version

```powershell
git tag v1.1.0
git push origin v1.1.0
```

Nothing is built when you push to `main`. A tag is what produces a build, and
the workflow then:

1. runs `go vet` and the test suite, so a release cannot be cut from a red tree;
2. makes sure NSIS is on the runner, installing it when the image does not
   carry it — Wails only warns when `makensis` is missing and still exits 0,
   which would otherwise produce a release with no installer in it;
3. builds with the tag stamped into the binary:
   `-ldflags "-X grabone/internal/appinfo.Version=1.1.0"`;
4. produces the NSIS installer and the portable executable;
5. writes `checksums.txt` with the SHA-256 of each file;
6. publishes a GitHub release with all three attached.

`ci.yml` runs the same checks on pull requests, and can be started by hand from
the Actions tab.

A pre-release is marked automatically: a tag containing a hyphen, such as
`v1.2.0-rc1`, is published as a pre-release and the updater treats it as older
than the final `v1.2.0`.

`workflow_dispatch` runs the same build without publishing, which is the way to
check the installer before tagging.

## What the release must contain

The updater inside the application will only install an update when the release
carries **both**:

| Asset | Why |
| --- | --- |
| `GrabOne-<version>-installer.exe` | The installer it runs |
| `checksums.txt` | The SHA-256 it verifies before running anything |

A release without a checksum file still shows in the application as available,
but the update has to be installed by hand from the release page. This is
deliberate: an installer that cannot be verified is never executed.

## Versioning

`internal/appinfo.Version` is the default for a local build and the fallback if a
binary is built without `-ldflags`. Keep it at the last released version so a
development build does not claim to be newer than it is.

The version appears in the window title, the header badge and Settings › About,
all from that one variable.

## Installer behaviour

The NSIS configuration in
[`build/windows/installer/project.nsi`](../build/windows/installer/project.nsi)
asks who to install for on its own wizard page. The default is **all users**,
landing in `C:\Program Files\GrabOne`; the other choice is the current user
only, in `%LOCALAPPDATA%\Programs\GrabOne`. Either way there is a Start-menu
entry, a desktop shortcut and an uninstaller.

The manifest stays unelevated whichever scope is chosen:

```nsis
!define REQUEST_EXECUTION_LEVEL "user"
```

That is what lets the application apply its own updates. The updater starts the
installer with `CreateProcess`, which cannot start a binary whose manifest asks
for elevation at all — it fails with `ERROR_ELEVATION_REQUIRED` before any
prompt appears. So the installer starts unelevated and re-launches itself
through `ShellExecute("runas")` once the all-users scope is confirmed. Declining
that prompt returns to the page rather than failing the install.

Windows locks a running image against writing, so installing or uninstalling
over an open GrabOne would fail on `GrabOne.exe` with nothing but NSIS's own
"error opening file for writing". The installer tests for that first, by opening
the executable for append, before anything is written.

Installing asks what to do about it, since the user may want to save work first:
Abort, Retry after closing GrabOne themselves, or Ignore to end the process with
`taskkill`. A silent run takes the Ignore path. Uninstalling closes it outright
without asking, because the application is being removed either way. Either path
gives up after two failed attempts rather than looping on a process that will
not end.

A self-update gets a short grace wait before any of this, since the application
starts the installer and only then closes.

Because the scope is only known at run time, the installer cannot use the
compile-time `WAILS_INSTALL_SCOPE` define or the `wails.setShellContext` and
`wails.writeUninstaller` macros. It sets `SetShellVarContext` itself and writes
its uninstall entries to `SHCTX`, so they land in `HKLM` or `HKCU` to match.
Re-running the installer reads the existing `InstallLocation` and pre-selects
that scope and folder, so an update replaces the current install instead of
adding a second one beside it.

## How the in-app update works

```text
Settings or startup
   ↓  GET api.github.com/repos/<owner>/<repo>/releases/latest
compare the tag with the running version
   ↓  a newer version → banner in the window
user chooses Download
   ↓  fetch checksums.txt, then the installer
verify SHA-256
   ↓  matches → keep, does not match → delete and report
user chooses Install and restart
   ↓  start the installer, close GrabOne
```

The rules the updater follows:

- only `https` addresses on GitHub hosts are fetched, including after redirects;
- the installer is downloaded to `%LOCALAPPDATA%\GrabOne\updates`;
- a file that fails verification is deleted rather than kept;
- only a file ending in `.exe` is ever started;
- nothing is downloaded or installed without the user asking for it, and the
  startup check can be turned off in Settings › Updates.

## Code signing

These builds are not signed, so Windows SmartScreen warns about an unrecognised
publisher on first run. To sign them, add a certificate to the repository
secrets and uncomment the signing lines in `project.nsi`:

```nsis
#!uninstfinalize 'signtool --file "%1"'
#!finalize 'signtool --file "%1"'
```

and sign `GrabOne.exe` in the workflow before the installer is built.

## Re-running a tag

A tag that failed before publishing can simply be moved:

```powershell
git tag -d v1.1.0
git push origin :refs/tags/v1.1.0
git tag -a v1.1.0 -m "GrabOne v1.1.0"
git push origin v1.1.0
```

The workflow that runs is the one committed at the tagged commit, so a fix to
the workflow itself has to be committed before the tag is recreated.

Once a release is published, prefer a new version over moving the tag: the
updater compares tags, and people may already have downloaded the old one.

## Local checks before tagging

```powershell
go test ./...
go vet ./...
gofmt -l .
cd frontend; npm run check
```

Building the installer locally needs [NSIS](https://nsis.sourceforge.io/) on
PATH:

```powershell
wails build -clean -platform windows/amd64 -nsis
```
