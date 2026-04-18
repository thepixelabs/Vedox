# Security Policy

## Reporting Vulnerabilities

Report security issues to security@pixelabs.dev (not via GitHub issues).

## Credential Management Rules

1. All credentials are stored in the OS keychain.
2. No plaintext credentials in source code or documentation.
3. Rotate all keys on suspected compromise.
4. Fine-grained GitHub PATs with 90-day expiry only.
5. Stripe live keys in production keychain only.

## What Not To Commit

The following file types are blocked by the secretscan pre-commit gate:
- *.env files
- *.pem, *.key, *.p12 files
- credentials.json
- service account JSON files
- Any file containing AWS, GitHub, Stripe, Slack, or Google API credentials

## Response Plan

1. Revoke the credential immediately.
2. Check audit log for unauthorized usage.
3. Rotate all related credentials.
4. Review Git history for additional leaks.
