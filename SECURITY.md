# Security policy

MKS is an admin panel: it handles authentication, authorization, file uploads, ZIP
packages and (optionally) commands and custom JavaScript for a public site. Please
report security issues privately.

## Reporting

- Email: **miransalehi.ms@gmail.com**
- Or use GitHub's private vulnerability reporting on the repository.
- Do not open a public issue for a vulnerability.

Include the affected version, a description, reproduction steps and, if possible,
a proof of concept. You will get an acknowledgement within 72 hours.

## Supported versions

| Version | Supported |
|---|---|
| 0.1.x | yes |
| < 0.1 | no |

## Scope worth testing

- Session and CSRF handling of the panel (`{panel}/api`).
- Permission checks: resource policies, the global gate, actions, widgets and pages.
- Multi-tenancy boundaries (a user must not read or write another tenant's records).
- Uploads and ZIP package installs (path traversal, size bombs, archive bombs).
- The console allowlist (no shell; plain arguments only) and the background runner.
- Custom CSS/JavaScript (off by default; only users with `cms.themes.code` and a
  plugin with `CustomJS: true` may set it).

## Disclosure

We will credit reporters in the release notes unless you ask to stay anonymous.
