## 1. Protocol Verification

- [ ] 1.1 Identify candidate Beijing realtime quote symbols for live sampling, including `920*`, `8*`, and `4*` code families.
- [ ] 1.2 Probe reachable TDX HQ servers and capture successful Beijing quote responses.
- [ ] 1.3 Verify the TDX market byte and response shape for explicit `bj` quote requests.
- [ ] 1.4 Verify whether TDX standard行情 security count/list supports Beijing market discovery.
- [ ] 1.5 Document the verified mapping and sample commands in the change notes or design document.

## 2. Quote Implementation

- [ ] 2.1 Add a Beijing TDX market code constant only after verification.
- [ ] 2.2 Update realtime quote request validation to accept explicit `bj:<symbol>`.
- [ ] 2.3 Update inferred realtime quote requests so `920*`, `8*`, and `4*` map to `bj`.
- [ ] 2.4 Update quote response market mapping so decoded Beijing responses return `market: "bj"`.
- [ ] 2.5 Keep unsupported-market errors clear if verification shows Beijing is not available through standard `hq`.

## 3. Security List and Sweep

- [ ] 3.1 Add Beijing support to security count/list request construction if the verified standard行情 path supports it.
- [ ] 3.2 Add Beijing security-list response tests using captured or synthetic fixtures.
- [ ] 3.3 Add `quote-sweep --market bj` support through the existing batch quote workflow.
- [ ] 3.4 Add quote sweep tests covering Beijing symbol discovery and explicit Beijing symbol lists.

## 4. Tests and Documentation

- [ ] 4.1 Add parser tests for explicit and inferred Beijing quote requests.
- [ ] 4.2 Add quote response decoding tests for Beijing market samples.
- [ ] 4.3 Add CLI tests for `marketd quote --symbol bj:<code>` and inferred Beijing symbols.
- [ ] 4.4 Update `docs/design/tdx-realtime-quotes.md` with verified Beijing behavior and limitations.
- [ ] 4.5 Run `go test ./...`.
- [ ] 4.6 Run `openspec validate --all`.
