# Security and trust boundaries

Storepath is an experimental local library, command-line tool, and browser demo. It evaluates
provided workload assumptions and produces recommendations and modeled costs.
It does not provision, move, read, or delete cloud objects or volumes. Runtime
commands require no cloud credentials, network service, or cloud account.

Treat input files and output as potentially sensitive. Use synthetic workloads
when filing issues or sharing results. The host OS, Go runtime, operator, input
files, and calling application are trusted. Storepath is not an isolation boundary
and has no authentication or tenant separation; those belong to its caller.
Recommendations are only as reliable as the inputs and the documented model.

The browser demo embeds the Go engine, runtime, and synthetic examples in one HTML
file. Evaluation makes no network requests and inputs are not uploaded. Exported
reports contain the supplied assumptions; review them before sharing. Download
links and source links leave the local demo only when selected. The browser and
downloaded HTML are trusted code, just as the native executable is.

## Release and installer boundaries

The installer downloads only the requested release from this GitHub repository
over HTTPS, requires one matching SHA-256 manifest entry, and extracts only the
expected regular-file binary to a private staging directory. The final rename is
atomic on the destination filesystem. Download, integrity, or extraction failures
preserve an existing executable. Tests run without network access using fixtures.

Checksums are hosted alongside the archives. They detect corruption and mismatched
downloads; they are not independent signatures. macOS packages are not Developer
ID signed or notarized. Use a source-build route approved by your device policy
if signed software is required; do not disable OS security protections.

The installer trusts GitHub, HTTPS, the local shell utilities, and the destination
filesystem. Do not install into directories writable by untrusted users. It is not
a defense against another process sharing your OS identity or a privileged host
attacker. Installing a new release is an explicit action; runtime commands never
self-update or download pricing.

## Public-source review

`python3 scripts/check-public-source.py` checks tracked source, the Git index, and
all locally reachable history for common credentials, private document types,
absolute home paths, and nonpublic email metadata. It fails on shallow history.
`STOREPATH_FORBIDDEN_STRINGS` optionally accepts a JSON array of additional private
strings. The scanner does not print matching source text.

The guard is a conservative pattern check, not a comprehensive secret detector.
Untracked files, unreachable objects, contextual proprietary information, and
unknown credential formats are outside its guarantees. Review the intended diff,
fixtures, prose, and commit metadata before publication. Revoke an exposed
credential at its issuer; deleting it from Git does not revoke it.

## Reporting

Use GitHub's **Security → Report a vulnerability** if private reporting is enabled.
Otherwise open an issue requesting a private contact route, without credentials,
private inputs, or exploit details against real systems. This experimental project
has no production-support or response-time SLA.
