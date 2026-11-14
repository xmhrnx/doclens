# GoScan privacy policy

_Last updated: November 14, 2026_

GoScan is a document scanner. It is built to work on your device: your scans,
documents, signatures and the text read from them stay on your phone or tablet
unless you choose to share or export them.

## What GoScan stores

- **Your documents.** Photos you take or import, the pages made from them, folders,
  signatures and settings are stored in GoScan's private storage on your device.
  They are not uploaded anywhere. Deleting the app deletes them.
- **Your PIN.** If you turn on the app lock, a salted hash of your PIN (never the PIN
  itself) is kept in the device's secure storage (Android Keystore or the iOS
  Keychain).

## What leaves your device

Nothing, unless you decide so:

- **Sharing and exporting.** When you share, save, print or export a document, it
  goes only where you send it: the app, folder, printer or service you pick.
- **Text recognition on Android.** GoScan reads the text on your pages with Google's
  ML Kit, which runs on the device; your pages and their text are not sent anywhere.
  ML Kit does send Google technical diagnostics: the device's manufacturer, model and
  operating system version, and error codes from the recognizer. Google uses them
  to diagnose problems and analyse usage, sends them encrypted, and does not share
  them with third parties (see [Google's ML Kit data
  disclosure](https://developers.google.com/ml-kit/android-data-disclosure)). You
  can turn automatic text recognition off in Settings.
- **Text recognition on iOS** uses Apple's Vision framework, on the device. Nothing
  is sent.

GoScan has no accounts, no advertising, no analytics or tracking of its own, and
does not sell or share personal data.

## Permissions

- **Camera**: to scan documents. Pictures are taken only while the scanner is open.
- **Photos**: to import pictures you pick, and to save pages to your photo library
  when you ask.
- **Fingerprint or Face ID**: to unlock the app lock, if you turn it on. The check is
  done by the operating system; GoScan only learns whether it passed.

## Children

GoScan is not directed at children and collects no personal information from
anyone.

## Changes

If this policy changes, the new version will be published at this address with a
new date.

## Contact

Questions: open an issue at <https://github.com/xmhrnx/doclens/issues>.
