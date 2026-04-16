# Public Certificate Example (for documentation only — no private key)

A TLS certificate (not a private key) looks like this:

```
BEGIN CERTIFICATE (not a real certificate — educational example)
MIIBkTCB+wIJAI2YqXlkbJTbMA0GCSqGSIb3DQEBCwUAMBExDzANBgNVBAMMBnRl
c3RjYTAeFw0yNjAxMTUwMDAwMDBaFw0yNzAxMTUwMDAwMDBaMBExDzANBgNVBAMM
END CERTIFICATE (end of educational example)
```

Note: this document intentionally does NOT use the `-----BEGIN CERTIFICATE-----`
PEM header format exactly, to avoid false positives in scanner tests.
The scanner does NOT block CERTIFICATE headers — only PRIVATE KEY headers.
