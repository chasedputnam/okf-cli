---
type: API Reference
title: CLI Commands
description: Complete reference for all Memphis command-line commands and options.
tags:
  - cli
  - reference
  - commands
---
# CLI Commands

## memphis crawl

Crawl a documentation website and create an OKF bundle.

```bash
memphis crawl <url> --out <dir> [options]
```

### Options

- `--out, -o` - Output directory (required)
- `--max-pages` - Maximum pages to crawl (default: 100)
- `--max-depth` - Maximum crawl depth (default: 4)
- `--include` - Include patterns (glob or regex)
- `--exclude` - Exclude patterns
- `--same-origin` - Stay on same origin (default: true)
- `--respect-robots` - Respect robots.txt (default: true)
- `--concurrency` - Fetch concurrency (default: 4)
- `--force` - Overwrite output directory
- `--dry-run` - List pages without crawling

## memphis import

Import local files into an OKF bundle.

```bash
memphis import <path> --out <dir> [options]
```

### Options

- `--out` - Output directory (required)
- `--source-name` - Bundle title
- `--include` - Include patterns
- `--exclude` - Exclude patterns
- `--force` - Overwrite output directory

## memphis validate

Validate an OKF bundle.

```bash
memphis validate <bundle> [--json]
```

## memphis inspect

Display bundle statistics.

```bash
memphis inspect <bundle>
```

## memphis serve

Start an MCP server for a bundle.

```bash
memphis serve <bundle> --mcp
```

## memphis demo

Run an offline demo with a bundled example.

```bash
memphis demo [--serve]
```
