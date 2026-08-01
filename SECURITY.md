# Security Policy

## Supported Versions

| Version | Supported          |
| ------- | ------------------ |
| 0.6.x   | :white_check_mark: |
| < 0.6   | :x:                |

## Reporting a Vulnerability

Please report privately through [GitHub's security advisories][advisories] rather
than in the open, since a repository holds a medical record.

[advisories]: https://github.com/TheDonDope/wits/security/advisories/new

## What a repository contains

A `.wits` directory is health data: what was dispensed, when, and how much was
used. It is created `0700` with its files `0600`, is never transmitted anywhere,
and the application makes no network calls at all.
