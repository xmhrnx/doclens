# GoScan store compliance answers

Answers for the stores' questionnaires, worked out from what the app does. You submit
them, so check each against the current version before you do.

## Google Play

### Data safety

GoScan's own code collects and shares nothing. On Android, text recognition uses
Google's ML Kit, which sends Google device details and error codes for diagnostics
and analytics ([Google's ML Kit data
disclosure](https://developers.google.com/ml-kit/android-data-disclosure)). Play
counts what SDKs in the app collect, so declare it:

| Question | Answer |
| --- | --- |
| Does your app collect or share any of the required user data types? | Yes |
| Is all of the user data collected by your app encrypted in transit? | Yes |
| Do you provide a way for users to request that their data is deleted? | No (no account; the data isn't linked to a person) |
| Data types collected | App info and performance → Diagnostics |
| — Collected or shared? | Collected (not shared) |
| — Processed ephemerally? | No |
| — Required or optional? | Optional (only when text recognition is used; it can be turned off in Settings) |
| — Purposes | Analytics |

Everything else (photos, files, documents, personal info, device IDs, location):
not collected. Pictures and documents stay on the device; sharing them is the user's
own action, which Play doesn't count as sharing.

### App content

| Section | Answer |
| --- | --- |
| Privacy policy | https://github.com/xmhrnx/doclens/blob/main/PRIVACY.md |
| Ads | No, the app contains no ads |
| App access | All functionality is available without special access (no login) |
| Target audience | 13 and older (not designed for children) |
| Content rating | Questionnaire: category "Utility, Productivity, Communication or Other"; answer No to violence, sexuality, language, controlled substances, gambling, user-generated content shared with others, location sharing, purchases. Expect "Everyone" / PEGI 3 |
| News app | No |
| Government app | No |
| Financial features | None |
| Health | No |
| Data safety | As above |

### Permissions

The bundle asks for camera, biometric (USE_BIOMETRIC) and internet. None is on
Play's list of permissions that need a declaration form. GoScan uses the system
photo picker, so it doesn't ask for READ_MEDIA_IMAGES.

### Technical requirements already met by the build

- Target SDK 36 (Android 16), required for new apps since August 31, 2026
  (`android.target_sdk` in giode.toml).
- 16 KB memory page support: native code is linked with 64 KB alignment.
- Android App Bundle (`.aab`) with native debug symbols, built by CI.

## App Store

### App Privacy ("nutrition label")

Data Not Collected. On iOS, text recognition is Apple's Vision framework on the
device; nothing leaves it unless the user shares a document. The build's privacy
manifest (`PrivacyInfo.xcprivacy`, from `[privacy]` in giode.toml) says the same,
and declares the file-timestamp and boot-time APIs Go uses.

### Age rating

Answer None to every content question: 4+.

### Export compliance (encryption)

GoScan uses standard AES-256 (in Go's standard library) only to password-protect
PDFs the user exports. When App Store Connect asks about a build:

- Does your app use encryption? **Yes**.
- Is it proprietary or non-standard? **No**: standard AES.
- Does it implement standard algorithms instead of, or in addition to, Apple's?
  **Yes**: Apple then treats it as mass-market encryption. The exemptions and
  reporting rules are export-law questions, so read Apple's
  [export compliance overview](https://developer.apple.com/documentation/security/complying-with-encryption-export-regulations)
  and answer for your situation. Distribution in France needs a separate declaration.

Once you know your answer, add `ITSAppUsesNonExemptEncryption` under `[ios.plist]` in
giode.toml so App Store Connect stops asking for every build.

### Review notes

- No account or login is needed; reviewers can scan any page.
- The camera is used only inside the scanner screen.
- iPad: the app runs full screen in portrait; tablet screenshots are in
  `store/graphics/tablet/`.

### Sign in with Apple, payments

Not applicable: no accounts, no purchases.
