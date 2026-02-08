# Encryption Key Setup Guide

## Overview

The Serenify backend uses AES-256-GCM encryption to protect sensitive recovery data (email addresses and phone numbers). This requires a 32-byte encryption key.

## Generating an Encryption Key

### Option 1: Using OpenSSL (Recommended)

```bash
openssl rand -base64 32
```

This will output a base64-encoded 32-byte key like:
```
K8j3mN9pQ2rT5vX7yZ1aB4cD6eF8gH0iJ2kL4mN6oP8qR0sT2uV4wX6yZ8=
```

### Option 2: Using Python

```python
import secrets
import base64
key = secrets.token_bytes(32)
print(base64.b64encode(key).decode())
```

### Option 3: Using Node.js

```javascript
const crypto = require('crypto');
const key = crypto.randomBytes(32);
console.log(key.toString('base64'));
```

## Setting the Encryption Key

### Local Development (.env file)

Add to your `serenify-backend/.env` file:

```env
ENCRYPTION_KEY=K8j3mN9pQ2rT5vX7yZ1aB4cD6eF8gH0iJ2kL4mN6oP8qR0sT2uV4wX6yZ8=
```

### Production (Environment Variables)

Set the `ENCRYPTION_KEY` environment variable in your hosting platform:

**Render.com:**
1. Go to your service settings
2. Navigate to "Environment"
3. Add new variable: `ENCRYPTION_KEY` = `<your-generated-key>`

**Other Platforms:**
- Set the `ENCRYPTION_KEY` environment variable with your generated key

## Important Notes

1. **Never commit the encryption key to version control**
2. **Keep the key secure** - if compromised, all encrypted data becomes readable
3. **Backup the key** - if lost, encrypted recovery emails cannot be decrypted
4. **Use different keys for different environments** (dev, staging, production)

## Verification

When the server starts, you should see:
- ✅ `Encryption key configured` - if key is valid
- ⚠️ `WARNING: ENCRYPTION_KEY not set` - if key is missing
- ⚠️ `WARNING: ENCRYPTION_KEY is invalid` - if key format is wrong

## What Happens Without Encryption Key?

- Users can still sign up and use the platform
- Recovery emails will NOT be stored (encryption will fail gracefully)
- Password reset and username recovery will not work via email
- All other features work normally

## Troubleshooting

### Error: "Failed to encrypt recovery email"

**Cause:** `ENCRYPTION_KEY` is not set or is invalid.

**Solution:**
1. Generate a new key using one of the methods above
2. Set it in your environment variables
3. Restart the server

### Error: "ENCRYPTION_KEY must decode to exactly 32 bytes"

**Cause:** The key is not base64-encoded or is the wrong length.

**Solution:**
1. Generate a new key using `openssl rand -base64 32`
2. Make sure you copy the entire key (it should be 44 characters including padding)
3. Set it in your environment variables

