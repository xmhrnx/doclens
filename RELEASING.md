# Releasing GoScan

Publishing a GitHub release builds the app and attaches the Android bundle and the iOS
IPA to it (`.github/workflows/release.yml`, written by `giode ci` from giode.toml).
`store-upload.yml` then sends them to Google Play and App Store Connect. Each store
step is skipped until its secrets are set, so releases work before the store accounts
exist.

| What | Value |
| --- | --- |
| Android package / iOS bundle ID | `com.xmhrnx.goscan` |
| iOS share extension bundle ID | `com.xmhrnx.goscan.share` |
| iOS App Group | `group.com.xmhrnx.goscan` |
| Version | the release tag (`v1.0.0` → 1.0.0) |
| Build number (versionCode / CFBundleVersion) | the workflow run number, so it always grows |

## 1. Android signing key (once)

```sh
GIODE_KEYSTORE_PASS='a long password' giode keystore
```

This writes `release.keystore` (ignored by git). Back it up together with its password
somewhere safe: with Play App Signing it is your upload key, and losing it means asking
Google to reset it. Then add two repository secrets (Settings → Secrets and variables →
Actions):

| Secret | Value |
| --- | --- |
| `ANDROID_KEYSTORE_BASE64` | `base64 -w0 release.keystore` |
| `GIODE_KEYSTORE_PASS` | the password |

Without them CI signs the bundle with a debug key, which Play rejects.

## 2. Google Play (once)

1. Create a developer account at <https://play.google.com/console/signup> (one-time
   fee, identity verification). New personal accounts must run a closed test with at
   least 12 testers for 14 days before they can publish to production.
2. Create the app: name "GoScan: PDF Document Scanner", default language, App, Free.
3. Fill in the store listing from [store/listing.md](store/listing.md) and the graphics
   in `store/graphics/`, and the App content and Data safety sections from
   [store/compliance.md](store/compliance.md).
4. Publish a release on GitHub (step 4 below), download its `.aab`, and upload it by
   hand to Testing → Internal testing. Accept Play App Signing when asked. Play only
   takes API uploads for an app that already has one bundle.
5. For automatic uploads: in Google Cloud, create a service account with a JSON key
   and enable the Google Play Android Developer API; in Play Console → Users and
   permissions, invite the service account's email with release permissions for
   GoScan. Store the JSON key's contents as the `PLAY_SERVICE_ACCOUNT_JSON` secret.

After that every release lands in internal testing as a draft; promote it to closed
testing or production in Play Console.

## 3. App Store (once)

Needs an Apple Developer Program membership.

1. Certificates, Identifiers & Profiles:
   - App IDs `com.xmhrnx.goscan` and `com.xmhrnx.goscan.share`, both with the App
     Groups capability and the group `group.com.xmhrnx.goscan`.
   - An Apple Distribution certificate. Export it with its key from Keychain Access as
     a `.p12`.
   - Two App Store provisioning profiles, one per App ID.
2. App Store Connect → Apps → New App: iOS, name "GoScan: PDF Document Scanner",
   bundle ID `com.xmhrnx.goscan`, SKU `goscan`. Fill in the listing and App Privacy
   from `store/`.
3. Users and Access → Integrations → App Store Connect API: create a key with the App
   Manager role and download the `.p8` file (it can be downloaded once).
4. Repository secrets:

| Secret | Value |
| --- | --- |
| `IOS_CERT_BASE64` | `base64 -i distribution.p12` |
| `IOS_CERT_PASS` | the `.p12` password |
| `IOS_PROFILE_BASE64` | `base64 -i GoScan_App_Store.mobileprovision` |
| `IOS_SHARE_PROFILE_BASE64` | `base64 -i GoScan_Share_App_Store.mobileprovision` |
| `IOS_TEAM_ID` | the 10-character team ID |
| `ASC_KEY_ID` | the API key's ID |
| `ASC_ISSUER_ID` | the issuer ID shown above the keys |
| `ASC_KEY_P8` | the contents of the `.p8` file |

Without the certificate and both profiles, CI only checks that the app builds for the
simulator and attaches no IPA.

Export compliance: decide your answer with [store/compliance.md](store/compliance.md),
then add it to giode.toml so App Store Connect stops asking each build, for example:

```toml
[ios.plist]
ITSAppUsesNonExemptEncryption = false
```

## 4. Publishing a release

1. Raise `version` in giode.toml if you like (the tag wins in CI), commit, and push.
2. On GitHub: Releases → Draft a new release, tag `v1.0.0` on `main`, publish.
3. `release` builds and attaches `GoScan-*.aab`, `native-debug-symbols.zip`, the APK
   and the IPA. `store upload` then sends the bundle to Play's internal track and the
   IPA to App Store Connect, where it appears in TestFlight after processing.
4. Submit for review in each console.

To upload an existing release again: Actions → store upload → Run workflow, with its
tag.

## Store checklist

- [ ] Privacy policy URL works: <https://github.com/xmhrnx/doclens/blob/main/PRIVACY.md>
      (the repository must be public, or host the file elsewhere and update the URL in
      `store/listing.md`)
- [ ] Listing text and graphics from `store/`
- [ ] Play: Data safety, content rating, target audience, ads declaration
- [ ] App Store: App Privacy, age rating, export compliance, review notes
- [ ] Test the release build on a real phone and iPad before submitting
