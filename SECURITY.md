# Security Policy

## Scope

cc-artex is an offensive security tool. Its purpose is to assist with authorized
penetration testing, so "it can be used to test systems" is by design, not a
vulnerability.

A security issue in cc-artex is a flaw in the tool itself that could harm its
users or the systems they run it on in unintended ways. Examples:

- Credential leakage: stored credentials exposed to unauthorized parties, leaked
  into logs, or transmitted insecurely.
- Remote code execution in the tool (not in the target): input from MCP clients,
  traffic data, or database content that leads to code execution on the
  operator's machine.
- Path traversal or file overwrite in the traffic recording subsystem.
- Authentication or authorization bypass in the HTTP transport.
- Denial of service against the operator's machine (resource exhaustion from
  crafted input).
- Dependency vulnerabilities that are reachable through cc-artex's code paths.

Not in scope: vulnerabilities in targets discovered using cc-artex, or the
intended ability to intercept, record, and replay network traffic.

## Reporting a vulnerability

**Do not open a public GitHub issue for security vulnerabilities.**

Report via one of:

- **Email:** [security@cc-artex.dev](mailto:security@cc-artex.dev). Encrypt with
  our PGP key if available (check the repository root or keyservers for
  `security@cc-artex.dev`).
- **GitHub Security Advisories:** use the "Report a vulnerability" button on the
  Security tab of the repository. This creates a private advisory visible only to
  maintainers.

Include:

1. A description of the vulnerability and its impact.
2. Steps to reproduce or a proof of concept.
3. The version or commit hash you tested against.
4. Your assessment of severity (optional but helpful).

## Response timeline

- **Acknowledgment:** within 72 hours of receipt.
- **Initial assessment:** within 7 days. We will confirm whether the report is
  accepted and provide an estimated timeline for a fix.
- **Fix release:** we aim to release a patch within 30 days of confirming a
  vulnerability, though complex issues may take longer. We will keep you informed
  of progress.
- **Disclosure:** we will coordinate disclosure timing with you. We ask for a
  90-day disclosure window from the date of the report, but we are open to
  negotiation.

## Credit

We are happy to credit reporters in release notes and the advisory unless you
prefer to remain anonymous. Let us know your preference when reporting.
