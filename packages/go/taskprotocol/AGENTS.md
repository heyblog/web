# Task Protocol

The API owns these internal HTTP DTOs, lifecycle constants and pure scheduling
rules. Both API and worker consume them. No database entities, repositories,
application imports, secrets or network clients belong here.

Increment ProtocolVersion when an incompatible worker contract changes.
`mise.toml` owns verification. Use five-field Cron and explicit IANA timezones.
