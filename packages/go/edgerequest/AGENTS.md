# Edge Request Library

Own public-target validation, byte-preserving Edge transport, bounded redirects,
timeouts and per-hop authorization. Receive endpoints, resolver and authorizer
from callers. Do not import applications, database code or business parsers.

`go.mod` owns dependencies; `mise.toml` owns check, test and verify commands.
Coordinate the wire contract with `apps/edge`. Tests use isolated HTTP servers;
they do not establish real provider behavior.
