# Releasing GrabOne

A release is produced by pushing a version tag. Everything else — building,
checksums, the GitHub release and the notes — happens in
[`.github/workflows/release.yml`](../.github/workflows/release.yml).

## Publishing a version

```powershell
git tag v1.1.0
git push origin v1.1.0
```

The workflow then:

1. runs `go vet` and the test suite, so a release cannot be cut from a red tree;
2. builds with the tag stamped into the binary:
   `-ldflags "-X grabone/internal/appinfo.Version=1.1.0"`;
3. produces the NSIS installer and the portable executable;
4. writes `checksums.txt` with the SHA-256 of each file;
5. publishes a GitHub release with all three attached.

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
installs **for the current user**:

```nsis
!define REQUEST_EXECUTION_LEVEL "user"
!define WAILS_INSTALL_SCOPE "user"
```

This matters for updating: a per-user install needs no administrator prompt, so
the running application can start the installer itself. A machine-wide install
would raise a UAC dialog that the application cannot answer.

GrabOne lands in `%LOCALAPPDATA%\Programs\GrabOne`, with a Start-menu entry and
an uninstaller.

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
