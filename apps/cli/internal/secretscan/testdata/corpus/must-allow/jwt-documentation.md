# JWT Authentication Guide

This document explains how to work with JSON Web Tokens.

## Structure

A JWT has three parts: header.payload.signature

Each part is base64url encoded. Example of what the DECODED header looks like:

```json
{"alg": "HS256", "typ": "JWT"}
```

Note: This document does NOT contain an actual JWT token value.
The base64url encoding of a real JWT starts with eyJ (the encoding of `{`).

## Validation

Always validate:
1. Signature (using the shared secret or public key)
2. Expiry (`exp` claim)
3. Issuer (`iss` claim)
4. Audience (`aud` claim)

Never trust the `alg: none` claim.
