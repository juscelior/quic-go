## Feature Comparison Matrix

| Feature                       | Prague | RFC9002 |
|-------------------------------|--------|---------|
| ECN-based control             | Yes    | Optional (RFC3168) |
| Uses ECT(1) (L4S identifier)  | Yes    | No      |
| Accurate ECN counters (ACK)   | Yes    | N/A     |
| Low-queue, scalable response  | Yes    | No      |
| RTT-independence (reduced)    | Yes    | No      |
| Loss response (Classic-safe)  | Yes    | Yes     |
| Pacing required               | Yes    | Usually |
| Startup burst control         | Yes    | Yes     |
| L4S coexistence friendliness  | Yes    | N/A     |
