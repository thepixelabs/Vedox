#!/bin/bash
# Application config — move to secrets manager
# This file has both a high-severity Stripe key AND generic secret assignments.

# This Stripe live key is HIGH severity and will block the commit:
export STRIPE_SECRET_KEY=sk_live_4eC39HqLyjWDarjtT1zdp7dcXXXXXXXXXX

# These are MEDIUM severity (advisory) and will be warned about:
export DATABASE_SECRET_KEY=thisIsAVeryLongSecretValueForTheDatabaseThatShouldBeInSecretsManager
export API_TOKEN=anotherLongTokenValueThatShouldNotBeInSourceCode123456789
export ADMIN_PASSWORD="super-secret-password-please-rotate-me-immediately"
