<!-- markdownlint-disable MD041 MD010 -->

## `up-mcp`

```diff
+ 📡 A Model Context Protocol (MCP) server for the Up Bank API.
```

<a href="LICENSE" target="_blank"><img src="https://img.shields.io/github/license/jmpa-io/up-mcp.svg" alt="GitHub License"></a>

## `Tools`

The following MCP tools are available:

- `ping` — ping the Up Bank API.
- `list_accounts` — list all Up Bank accounts.
- `list_transactions` — list transactions across all accounts.
- `list_transactions_by_account` — list transactions for a specific account.
- `get_transaction` — get a single transaction by ID.
- `list_categories` — list all Up Bank categories.
- `set_transaction_category` — set or remove the category on a transaction.
- `list_tags` — list all tags.
- `add_tags` — add tags to a transaction.
- `remove_tags` — remove tags from a transaction.

## `Usage`

```bash
UP_TOKEN=xxx go run .
```
