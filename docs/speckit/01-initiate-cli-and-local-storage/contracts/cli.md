# CLI Contract: Initialize CLI and Local Storage

## Executable

```text
wl [flags] [command]
```

## Commands in This Feature

| Invocation | Behavior | Storage required |
|---|---|---:|
| `wl` | Ensure default config and SQLite schema exist; report readiness | Yes |
| `wl --help`, `wl -h`, `wl help` | Display command usage and available commands | No |
| `wl --version`, `wl version` | Display build version (`dev` when not injected) | No |

Future command names shown in help may be listed as planned only when registered; this feature does not promise their behavior.

## Exit Status

| Status | Meaning |
|---:|---|
| `0` | Command completed successfully |
| Non-zero | Invalid invocation, configuration error, home/path failure, SQLite error, or migration failure |

Errors on storage initialization should include the failed operation and relevant path. Do not print secrets, config values unrelated to the error, or credentials.

## First-Run Files

```text
~/.worklog/
├── config.yaml
└── worklog.db
```

On POSIX platforms, request mode `0700` for the directory and `0600` for newly created files. Existing file modes are not silently broadened. A database may temporarily have SQLite journal sidecar files while open.

## Default Configuration

```yaml
database:
  path: ~/.worklog/worklog.db

ticket:
  pattern: "[A-Z][A-Z0-9]+-[0-9]+"

git:
  capture_changed_files: true
  capture_diff_stat: true
  capture_full_diff: false

ai:
  enabled: true
  include_diff: false
```

No provider key or other credential appears in this file by default. AI configuration does not initiate a network request during bootstrap.

## Version Format

Release builds may inject a semantic version and optional commit identifier. Local builds without metadata print `wl version dev`. Version output must be deterministic for the same build.

## Compatibility

- Data directory: user's home directory plus `.worklog`.
- Initial supported operating systems: macOS and Linux.
- Config and database paths must use platform-aware path joining.
- Help and version must not attempt to open configuration or SQLite.
