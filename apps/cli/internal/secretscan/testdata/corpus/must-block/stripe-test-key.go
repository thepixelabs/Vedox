package payments

import "github.com/stripe/stripe-go/v76"

func init() {
    // Test key — safe to expose? NO — reveals API structure.
    stripe.Key = "sk_test_4eC39HqLyjWDarjtT1zdp7dcXXXXXXXXXX"
}
