# Security Policy

## Supported versions

Only the latest release is supported. Every fix ships as a new release rather
than a backport, so upgrading is the fix:

```sh
go install github.com/TheDonDope/wits/cmd/wits@latest
```

## Reporting a vulnerability

Please report privately through [GitHub's security advisories][advisories]
rather than in the open, since a repository holds a medical record.

[advisories]: https://github.com/TheDonDope/wits/security/advisories/new

## What a repository contains

A `.wits` directory is health data: what was dispensed, when, and how much was
used. It is created `0700` with its files `0600`, is never transmitted anywhere,
and the application makes no network calls at all.

The one input wits reads from outside its own repository is the spreadsheet
handed to `wits import`. Malformed workbooks are the library's concern
(excelize), and its advisories are tracked by Dependabot.
