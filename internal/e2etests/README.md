# E2E Bison Relay Testing

This package contains a set of E2E tests for the client side of Bison Relay.

## Full Logging

Run with the `BR_E2E_LOG=1` env variable set to show full log for tests:

```
$ BR_E2E_LOG=1 go test -v .
```

That env var can also be set with the name of the test to only show logs for a
specific test:

```
$ BR_E2E_LOG=TestBasicGCFeatures go test -v .
```

