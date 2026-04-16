# Environment Variables Reference

Copy this to `.env` and fill in your values.

```
# Database
DATABASE_HOST=localhost
DATABASE_PORT=5432
DATABASE_NAME=myapp

# Vedox daemon
VEDOX_PORT=7474
VEDOX_WORKSPACE=/home/user/workspace

# The following must be set in your keychain, NOT in .env:
# - ANTHROPIC_API_KEY
# - GITHUB_TOKEN
# - STRIPE_SECRET_KEY
```

Never put real secrets in `.env.example`. Use placeholder strings.
