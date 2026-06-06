## 1. Protocol Verification

- [x] 1.1 Identify candidate Beijing realtime quote symbols for live sampling, including `920*`, `8*`, and `4*` code families.
- [x] 1.2 Probe reachable TDX HQ servers and capture successful Beijing quote responses.
- [x] 1.3 Verify the TDX market byte and response shape for explicit `bj` quote requests.
- [x] 1.4 Verify whether TDX standard行情 security count/list supports Beijing market discovery.
- [x] 1.5 Document the verified mapping and sample commands in the change notes or design document.

## 2. Quote Implementation

- [x] 2.1 Add a Beijing TDX market code constant only after verification.
- [x] 2.2 Update realtime quote request validation to accept explicit `bj:<symbol>`.
- [x] 2.3 Update inferred realtime quote requests so `920*`, `8*`, and `4*` map to `bj`.
- [x] 2.4 Update quote response market mapping so decoded Beijing responses return `market: "bj"`.
- [x] 2.5 Keep unsupported-market errors clear if verification shows Beijing is not available through standard `hq`.

## 3. Security List and Sweep

- [x] 3.1 Keep Beijing security count/list discovery disabled because the verified standard行情 path did not support it.
- [x] 3.2 Add Beijing security-list unsupported tests for the unavailable discovery path.
- [x] 3.3 Add explicit Beijing symbol support through the existing quote sweep workflow.
- [x] 3.4 Add quote sweep tests covering explicit Beijing symbol lists and unsupported Beijing discovery.

## 4. Tests and Documentation

- [x] 4.1 Add parser tests for explicit and inferred Beijing quote requests.
- [x] 4.2 Add quote response decoding tests for Beijing market samples.
- [x] 4.3 Add CLI tests for `marketd quote --symbol bj:<code>` and inferred Beijing symbols.
- [x] 4.4 Update `docs/design/tdx-realtime-quotes.md` with verified Beijing behavior and limitations.
- [x] 4.5 Run `go test ./...`.
- [x] 4.6 Run `openspec validate --all`.
