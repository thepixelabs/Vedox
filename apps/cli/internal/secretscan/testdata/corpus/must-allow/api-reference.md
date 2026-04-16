# Vedox API Reference

## POST /api/projects/:id/docs

Creates or updates a document.

### Request Headers

| Header | Description |
|--------|-------------|
| X-Vedox-Key-Id | The API key UUID |
| X-Vedox-Signature | HMAC-SHA256 hex digest of the request body |
| X-Vedox-Timestamp | Unix timestamp (seconds, UTC) |

### Example Request

```bash
curl -X POST http://localhost:7474/api/projects/my-project/docs \
  -H "X-Vedox-Key-Id: 550e8400-e29b-41d4-a716-446655440000" \
  -H "X-Vedox-Signature: <computed-hmac>" \
  -H "X-Vedox-Timestamp: 1716300000" \
  -d @doc.json
```

Note: The signature above is a placeholder. Real signatures are computed
by the agent SDK using the keychain-stored secret.
