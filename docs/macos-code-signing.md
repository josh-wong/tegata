# macOS code signing setup

This guide walks through setting up code signing for Tegata's macOS binaries and GUI app. The release workflow automatically signs and notarizes binaries when you push a version tag, but the initial setup requires several one-time steps in Apple Developer Account and GitHub.

## Initial setup

You'll need to set up:

1. A **Developer ID Application certificate** (for code signing)
2. A **Developer ID Installer certificate** (optional, for notarization)
3. **App-specific password** (for notarization)
4. GitHub **Secrets** for the workflow

### Step 1: Create/obtain Developer ID certificate

1. Go to [Apple Developer Account](https://developer.apple.com/account).
2. Sign in with your Apple ID.
3. Navigate to **Certificates, Identifiers & Profiles** → **Certificates**.
4. Click the **+** button to create a new certificate
5. Select **Developer ID Application** (this is for code signing the app).
6. Follow the prompts:
   - Download the `.certSigningRequest` file from your Mac (or create one using Keychain Access).
   - Upload it to Apple.
   - Download the resulting `.cer` file.
7. Double-click the `.cer` file to import it into Keychain Access.

### Step 2: Export certificate as .p12

1. Open **Keychain Access** on your Mac.
2. Find your new "Developer ID Application" certificate (look in the **Certificates** category).
3. Right-click it → **Export**.
4. Save as `developer-id.p12`.
5. Set a strong password when prompted (you'll need this password in Step 4).
6. Note the password you used.

### Step 3: Encode .p12 as base64

In Terminal, run:

```bash
base64 -i ~/Downloads/developer-id.p12 | pbcopy
```

This copies the base64-encoded certificate to your clipboard.

### Step 4: Create app-specific password

1. Go to [appleid.apple.com](https://appleid.apple.com) and sign in.
2. Navigate to **Security** → **App passwords**.
3. Select **macOS** as the app type.
4. Generate a new app-specific password.
5. Copy the 16-character password (spaces don't matter).

### Step 5: Get your Team ID

1. Go to [Apple Developer Account](https://developer.apple.com/account).
2. Click your name in the top-right corner → **View Account**.
3. Under **Membership**, find your **Team ID** (looks like `AB123CD456`).

### Step 6: Set GitHub secrets

In your GitHub repo, go to **Settings** → **Secrets and variables** → **Actions** and create these secrets:

| Secret Name                              | Value                                               |
|------------------------------------------|-----------------------------------------------------|
| `APPLE_DEVELOPER_CERTIFICATE_P12_BASE64` | The base64 string from Step 3                       |
| `APPLE_DEVELOPER_CERTIFICATE_PASSWORD`   | The password you set in Step 2                      |
| `APPLE_ID`                               | Your full Apple ID (for example, `you@example.com`) |
| `APPLE_TEAM_ID`                          | Your Team ID from Step 5                            |
| `APPLE_APP_SPECIFIC_PASSWORD`            | The 16-char password from Step 4                    |

### Verification

To verify everything is set up correctly:

1. Push a tag (for example, `git tag v0.1.0 && git push --tags`).
2. Watch the **Release** workflow run.
3. Check the **macos-signing** job to confirm it succeeds.

If code signing fails, check:

- The certificate password matches what you exported.
- The app-specific password is correct (16 characters, no spaces).
- Your Team ID is exactly right.
- The certificate is valid and not expired.

## Cleanup/removal

If you need to start over or remove what you've added to your local environment, follow these steps:

### Step 1: Delete temporary files

Remove the `.p12` and `.cer` files from your Mac:

```bash
rm ~/Downloads/developer-id.p12
rm ~/Downloads/developer-id.cer
# Also remove any .certSigningRequest files if you created them
rm ~/Downloads/*.certSigningRequest
```

### Step 2: Remove certificate from Keychain

1. Open **Keychain Access** on your Mac.
2. Go to the **Certificates** category.
3. Find your "Developer ID Application" certificate.
4. Right-click it → **Delete**.
5. Confirm the deletion.

### Step 3: Remove GitHub secrets

In your GitHub repo:

1. Go to **Settings** → **Secrets and variables** → **Actions**
2. Delete each of these secrets:
   - `APPLE_DEVELOPER_CERTIFICATE_P12_BASE64`
   - `APPLE_DEVELOPER_CERTIFICATE_PASSWORD`
   - `APPLE_ID`
   - `APPLE_TEAM_ID`
   - `APPLE_APP_SPECIFIC_PASSWORD`

### Step 4: (Optional) Revoke certificate from Apple

If you want to fully revoke the certificate from Apple's side:

1. Go to [Apple Developer Account](https://developer.apple.com/account).
2. Navigate to **Certificates, Identifiers & Profiles** → **Certificates**.
3. Find your "Developer ID Application" certificate.
4. Click it and select **Revoke**.
5. Confirm the revocation.

## Starting over

Once you've completed the cleanup, you can start from **Step 1** of the Initial setup section again if needed.
