# Releasing DocLens

Publishing a GitHub release with a tag like `v1.0.0` runs
`.github/workflows/release.yml`. The tag sets the version and the workflow's
run number the build number. The workflow runs `go vet` and `go test`, builds
the Android and iOS packages and attaches them to the release. It is written
by the Giode CLI from `giode.toml`: after changing `[ci]` or a store section,
run `giode ci -f` instead of editing it, then redo the two edits the comment
above `[ci]` describes (pin the CLI version, take Go's version from `go.mod`).

DocLens is a phone app (`platforms = ["android", "ios"]`), so it goes to two
stores:

| Where | What the release gets | Needs |
| --- | --- | --- |
| Google Play | `DocLens.aab`, signed with the upload key | a Play Console account and the keystore secrets |
| Apple App Store (iPhone and iPad) | `DocLens.ipa` to upload with Transporter | an Apple Developer account, the distribution certificate and an App Store profile |

There is no Microsoft Store or Mac App Store package: DocLens is built for
phones only (a camera scanner with a touch UI), so `windows` and `macos` are
not in its platforms. Without the signing secrets the Android job still
builds an AAB signed with the debug key (Play refuses it), and the iOS job
only checks that the app builds for the simulator.

The privacy policy every store asks for is on the website:
<https://xmhrnx.github.io/doclens/#privacy>.

## Repository secrets

Set these in GitHub under Settings → Secrets and variables → Actions. Each
`*_BASE64` secret is the file encoded with `base64 -i file | pbcopy` on macOS,
or `base64 -w0 file` on Linux. Without a secret, that step is skipped and the
rest of the release still builds.

| Secret | What it is |
| --- | --- |
| `ANDROID_KEYSTORE_BASE64`, `GIODE_KEYSTORE_PASS` | The upload keystore (`release.keystore`) and its password: signs the `.aab` |
| `IOS_CERT_BASE64`, `IOS_CERT_PASS` | The "Apple Distribution" certificate as a `.p12`, and its password |
| `IOS_PROFILE_BASE64` | The "App Store Connect" provisioning profile for `com.xmhrnx.doclens` (`.mobileprovision`) |
| `IOS_TEAM_ID` | Your Apple team ID (10 characters, shown under Membership) |

`giode.toml` sets `android.keystore = "release.keystore"`: the workflow writes
that file from `ANDROID_KEYSTORE_BASE64`, and `giode build -release` refuses to
sign without the setting. `*.keystore` is in `.gitignore`; never commit it.

## Google Play

1. Open a Google Play Console developer account.
2. Create the upload key once, on your computer, in this folder:

   ```sh
   giode keystore            # writes release.keystore, asks for a password
   ```

   Keep `release.keystore` and its password somewhere safe (a password
   manager): every future update must be signed with it. Add both as the
   `ANDROID_KEYSTORE_BASE64` and `GIODE_KEYSTORE_PASS` secrets.
3. In Play Console, create the app "DocLens" (App, Free). The package name is
   `com.xmhrnx.doclens` (the `app.id` in `giode.toml`); it can never change.
4. Publish a release on GitHub. Upload the `DocLens.aab` it gets to a testing
   track (internal testing first). On the first upload, accept **Play App
   Signing**: Google keeps the app signing key and `release.keystore` stays
   your upload key. If it's ever lost, Play support can reset the upload key.
5. Fill in the listing and the App content pages with the answers below, then
   promote the release to production. New personal developer accounts must
   first run a closed test with at least 12 testers for 14 days.

**Target API level.** Play takes new apps and updates only when they target
Android 16 (API 36), since 31 August 2026; the CLI's default is 35, so
`giode.toml` sets `target_sdk = 36`. Targeting 36 turns on predictive back,
under which Android no longer calls the `onBackPressed` that Gio uses for
`giode.HandleBack`, so `giode.toml` also opts out with
`enableOnBackInvokedCallback = "false"`. Check the back gesture on an
Android 16 phone before each release: it must close sheets and go back a
screen, not leave the app.

**Permissions** in the bundle: `CAMERA` (scanning, optional hardware, so
tablets without a camera can install it), `USE_BIOMETRIC` (the app lock),
and what ML Kit's libraries add (network access for their diagnostics).
None needs a Play permission declaration. Check the final list in Play
Console under the bundle's details.

## Apple App Store

1. Join the Apple Developer Program.
2. In Certificates, Identifiers & Profiles:
   - Register the App ID `com.xmhrnx.doclens` (the `app.id` in `giode.toml`).
     It needs no extra capabilities.
   - Create an "Apple Distribution" certificate, export it from Keychain
     Access as a `.p12` with a password.
   - Create an "App Store Connect" provisioning profile for the App ID and
     that certificate.
3. Add `IOS_CERT_BASE64`, `IOS_CERT_PASS`, `IOS_PROFILE_BASE64` and
   `IOS_TEAM_ID`.
4. In App Store Connect, create the app: platform iOS, name "DocLens",
   bundle ID `com.xmhrnx.doclens`, a SKU such as `doclens`.
5. Publish a release on GitHub. It gets `DocLens.ipa`; upload it with the
   Transporter app, test it in TestFlight, then submit it for review with the
   listing below.

The build already carries what review checks for: the usage texts for the
camera, picked photos, saving to Photos and Face ID; a privacy manifest
(`PrivacyInfo.xcprivacy`, nothing collected, no tracking); and the export
compliance answer (`ITSAppUsesNonExemptEncryption = false`). The app icon is
`assets/icon-ios.png`: opaque and without an alpha channel, as App Store
Connect requires (`assets/icon-source/make.py` makes it with the others).

## Store listing

Accurate to the code; paste as is.

**Name:** DocLens

**Subtitle (Apple, ≤30):** Scan documents to clean PDFs

**Short description (Play, ≤80):** Scan paper into clean, searchable PDFs with filters, signatures and passwords.

**Keywords (Apple, ≤100):** `scanner,scan,pdf,document,ocr,receipt,sign,signature,id card,book,text,crop,camera,paper,notes`

**Description:**

> DocLens turns paper into clean PDFs with your phone's camera. It finds the
> page as you aim, takes the picture once the page holds still, flattens it and
> cleans away shadows. Everything happens on your phone.
>
> SCAN
> - Live page tracking with automatic capture, torch and tap to focus
> - Batch scanning, ID cards (front and back on one sheet at real size) and
>   books (a two-page spread split in two)
> - Import pictures from your photo library, or the pages of existing PDFs
>
> CLEAN UP
> - Magic Color, No Shadow, Grayscale, B&W and Original filters, with
>   brightness and contrast
> - Crop with draggable corners, a magnifier and snapping; rotate and retake
> - Every edit is non-destructive
>
> ORGANIZE
> - Reorder, delete with undo, extract pages into a new document, merge
>   documents
> - Folders, favorites and search
>
> SIGN AND MARK UP
> - Pen and highlighter
> - Draw your signature once and place it on any page
>
> TEXT
> - Text is recognized on the device: search finds words inside your pages,
>   copy or share a page's text, and PDFs get a searchable text layer
>
> EXPORT
> - PDF in Auto, A4, Letter or Legal, with margins and three quality levels
> - Password protection (AES-256) and watermarks
> - JPEG pages or one long image
> - Share, save to Files, save to Photos or print
>
> PRIVATE
> - No account, no ads, no tracking
> - Lock the app with a PIN, fingerprint or Face ID
>
> DocLens is open source: github.com/xmhrnx/doclens

**Category:** Productivity (Apple secondary: Business; Play: Productivity).

**Website / support URL:** https://xmhrnx.github.io/doclens/ (support:
https://github.com/xmhrnx/doclens/issues).

**Privacy policy URL:** https://xmhrnx.github.io/doclens/#privacy

**Age rating:** no objectionable content, no user-to-user communication, no
web browsing, no purchases, no ads. Apple: answer "None" everywhere (4+).
Play (IARC questionnaire, category "Utility, Productivity, Communication or
Other"): no to everything (Everyone / PEGI 3). Target audience: 13 and older;
the app is not designed for children.

**Apple App Privacy:** "Data Not Collected". On iOS everything stays on the
device: Apple's Vision reads the text, and the app has no server, analytics or
crash reporting.

**Play Data safety:** the app itself sends nothing, but Google's ML Kit (the
on-device text recognition) sends Google diagnostics, and Play counts what an
SDK collects ([ML Kit's disclosure](https://developers.google.com/ml-kit/android-data-disclosure)).
Answer:

- Does your app collect or share any of the required user data types? **Yes.**
- Is all of the user data encrypted in transit? **Yes.**
- Do you provide a way for users to request that their data is deleted? **No**
  (no account; the developer holds no data).
- Collected, not shared, not processed ephemerally, required:
  - **App info and performance → Diagnostics** (ML Kit's performance
    metrics, error codes, API configuration). Purpose: Analytics.
  - **Device or other IDs** (ML Kit's per-installation identifier). Purpose:
    Analytics.
- Nothing else: documents, photos, recognized text, signatures and settings
  stay on the device.
- Ads: **No**. Government app, financial features, health: **No**.

**Export compliance (Apple):** answered in the build. DocLens uses Apple's own
encryption (the Keychain holds the app lock) and AES-256, a standard
algorithm, for PDF passwords; it makes no network connections of its own: exempt, no documentation needed. If you make
the app available in France, App Store Connect may still ask for the French
encryption declaration.

**Screenshots:** the repository has none yet; take them on devices (or the
simulator / emulator) with a few sample documents in the library:

1. The live scanner with the page outline tracked (a real document on a desk).
2. The crop screen with the magnifier.
3. The filter strip on a page (Magic Color vs. Original).
4. The library with folders and favorites.
5. A page with a placed signature.
6. The export sheet (PDF size, password, watermark).

Apple needs 6.9" iPhone (1320 × 2868) and, because the app runs on iPad,
13" iPad (2064 × 2752) screenshots. Play needs at least two phone screenshots
and a 1024 × 500 feature graphic; `site/img/icon.png` (512 px) is the icon.

## Known limitations

- No Android SDK or Xcode in the environment where this was prepared, so the
  AAB and IPA are first built by the workflow. Check its run on the first
  release, and test the back gesture on Android 16 (see Target API level).
- Text recognition on Android reads Latin scripts only (ML Kit's bundled
  model); iOS's Vision reads more.
