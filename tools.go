package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	up "github.com/jmpa-io/up-go"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// registerTools registers all Up Bank MCP tools with the server.
func registerTools(server *mcp.Server, client *up.Client) {

	mcp.AddTool(server, &mcp.Tool{
		Name:        "ping",
		Description: "Pings the Up API to verify the token is valid and the service is reachable.",
	}, pingTool(client))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_accounts",
		Description: "Lists all Up Bank accounts (spending, savers, 2Up joint accounts).",
	}, listAccounts(client))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_transactions",
		Description: "Lists transactions across all accounts. Supports filtering by status, date range, category, and tag.",
	}, listTransactions(client))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_transactions_by_account",
		Description: "Lists transactions for a specific Up account by its ID. Supports the same filters as list_transactions.",
	}, listTransactionsByAccount(client))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_transaction",
		Description: "Retrieves a single Up transaction by its ID, including the real timestamp, rawText, tags, and category.",
	}, getTransaction(client))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_categories",
		Description: "Lists all Up Bank categories and their parent/child relationships.",
	}, listCategories(client))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "set_transaction_category",
		Description: "Sets the Up category on a transaction. Pass an empty category_id to de-categorise.",
	}, setTransactionCategory(client))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "list_tags",
		Description: "Lists all tags currently in use across all transactions.",
	}, listTags(client))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "add_tags",
		Description: "Adds one or more tags to a transaction. Up supports up to 6 tags per transaction. Duplicate tags are silently ignored.",
	}, addTags(client))

	mcp.AddTool(server, &mcp.Tool{
		Name:        "remove_tags",
		Description: "Removes one or more tags from a transaction. Tags not present are silently ignored.",
	}, removeTags(client))
}

// --- helpers ---

func toJSON(v any) string {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Sprintf("(marshal error: %v)", err)
	}
	return string(b)
}

func textResult(s string) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: s},
		},
	}
}

// --- Tool: ping ---

type pingInput struct{}

func pingTool(client *up.Client) func(context.Context, *mcp.CallToolRequest, pingInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, _ pingInput) (*mcp.CallToolResult, any, error) {
		p, err := client.Ping(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("ping failed: %w", err)
		}
		return textResult(fmt.Sprintf("Up API is reachable. Status: %s (id: %s)", p.Meta.StatusEmoji, p.Meta.ID)), nil, nil
	}
}

// --- Tool: list_accounts ---

type listAccountsInput struct {
	AccountType   string `json:"account_type,omitempty"    jsonschema:"optional filter: SAVER, TRANSACTIONAL, or HOME_LOAN"`
	OwnershipType string `json:"ownership_type,omitempty"  jsonschema:"optional filter: INDIVIDUAL or JOINT"`
}

func listAccounts(client *up.Client) func(context.Context, *mcp.CallToolRequest, listAccountsInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input listAccountsInput) (*mcp.CallToolResult, any, error) {
		var opts []up.ListAccountsOption
		if input.AccountType != "" {
			opts = append(opts, up.ListAccountsOptionFilterAccountType(up.AccountType(input.AccountType)))
		}
		if input.OwnershipType != "" {
			opts = append(opts, up.ListAccountsOptionFilterAccountOwnershipType(up.AccountOwnershipType(input.OwnershipType)))
		}
		accounts, err := client.ListAccounts(ctx, opts...)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to list accounts: %w", err)
		}
		return textResult(toJSON(accounts)), nil, nil
	}
}

// --- Tool: list_transactions ---

type listTransactionsInput struct {
	Status   string `json:"status,omitempty"    jsonschema:"optional filter: HELD or SETTLED"`
	Since    string `json:"since,omitempty"     jsonschema:"optional start datetime in RFC3339 format e.g. 2024-01-01T00:00:00+10:00"`
	Until    string `json:"until,omitempty"     jsonschema:"optional end datetime in RFC3339 format"`
	Category string `json:"category,omitempty"  jsonschema:"optional Up category ID e.g. restaurants-and-cafes"`
	Tag      string `json:"tag,omitempty"       jsonschema:"optional tag label to filter by"`
	PageSize int    `json:"page_size,omitempty" jsonschema:"number of results per page (max 100, default 20)"`
}

func buildTransactionOpts(input listTransactionsInput) ([]up.ListTransactionsOption, error) {
	var opts []up.ListTransactionsOption
	if input.Status != "" {
		opts = append(opts, up.ListTransactionsOptionStatus(up.TransactionStatus(input.Status)))
	}
	if input.Since != "" {
		t, err := time.Parse(time.RFC3339, input.Since)
		if err != nil {
			return nil, fmt.Errorf("invalid since format (expected RFC3339, e.g. 2024-01-01T00:00:00+10:00): %w", err)
		}
		opts = append(opts, up.ListTransactionsOptionSince(t))
	}
	if input.Until != "" {
		t, err := time.Parse(time.RFC3339, input.Until)
		if err != nil {
			return nil, fmt.Errorf("invalid until format (expected RFC3339): %w", err)
		}
		opts = append(opts, up.ListTransactionsOptionUntil(t))
	}
	if input.Category != "" {
		opts = append(opts, up.ListTransactionsOptionCategory(input.Category))
	}
	if input.Tag != "" {
		opts = append(opts, up.ListTransactionsOptionTag(input.Tag))
	}
	if input.PageSize > 0 {
		opts = append(opts, up.ListTransactionsOptionPageSize(input.PageSize))
	}
	return opts, nil
}

func listTransactions(client *up.Client) func(context.Context, *mcp.CallToolRequest, listTransactionsInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input listTransactionsInput) (*mcp.CallToolResult, any, error) {
		opts, err := buildTransactionOpts(input)
		if err != nil {
			return nil, nil, err
		}
		txns, err := client.ListTransactions(ctx, opts...)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to list transactions: %w", err)
		}
		return textResult(fmt.Sprintf("%s\n\n(total: %d)", toJSON(txns), len(txns))), nil, nil
	}
}

// --- Tool: list_transactions_by_account ---

type listTransactionsByAccountInput struct {
	AccountID string `json:"account_id"          jsonschema:"required,the Up account ID"`
	Status    string `json:"status,omitempty"    jsonschema:"optional filter: HELD or SETTLED"`
	Since     string `json:"since,omitempty"     jsonschema:"optional start datetime in RFC3339 format"`
	Until     string `json:"until,omitempty"     jsonschema:"optional end datetime in RFC3339 format"`
	Category  string `json:"category,omitempty"  jsonschema:"optional Up category ID"`
	Tag       string `json:"tag,omitempty"       jsonschema:"optional tag label to filter by"`
	PageSize  int    `json:"page_size,omitempty" jsonschema:"number of results per page (max 100)"`
}

func listTransactionsByAccount(client *up.Client) func(context.Context, *mcp.CallToolRequest, listTransactionsByAccountInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input listTransactionsByAccountInput) (*mcp.CallToolResult, any, error) {
		// reuse the same option builder
		baseInput := listTransactionsInput{
			Status:   input.Status,
			Since:    input.Since,
			Until:    input.Until,
			Category: input.Category,
			Tag:      input.Tag,
			PageSize: input.PageSize,
		}
		opts, err := buildTransactionOpts(baseInput)
		if err != nil {
			return nil, nil, err
		}
		txns, err := client.ListTransactionsByAccount(ctx, input.AccountID, opts...)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to list transactions for account %s: %w", input.AccountID, err)
		}
		return textResult(fmt.Sprintf("%s\n\n(total: %d)", toJSON(txns), len(txns))), nil, nil
	}
}

// --- Tool: get_transaction ---

type getTransactionInput struct {
	TransactionID string `json:"transaction_id" jsonschema:"required,the Up transaction ID"`
}

func getTransaction(client *up.Client) func(context.Context, *mcp.CallToolRequest, getTransactionInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input getTransactionInput) (*mcp.CallToolResult, any, error) {
		txn, err := client.GetTransaction(ctx, input.TransactionID)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to get transaction %s: %w", input.TransactionID, err)
		}
		return textResult(toJSON(txn)), nil, nil
	}
}

// --- Tool: list_categories ---

type listCategoriesInput struct{}

func listCategories(client *up.Client) func(context.Context, *mcp.CallToolRequest, listCategoriesInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, _ listCategoriesInput) (*mcp.CallToolResult, any, error) {
		cats, err := client.ListCategories(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to list categories: %w", err)
		}
		return textResult(toJSON(cats)), nil, nil
	}
}

// --- Tool: set_transaction_category ---

type setTransactionCategoryInput struct {
	TransactionID string `json:"transaction_id"        jsonschema:"required,the Up transaction ID"`
	CategoryID    string `json:"category_id,omitempty" jsonschema:"the Up category ID to assign (e.g. restaurants-and-cafes). Leave empty to de-categorise."`
}

func setTransactionCategory(client *up.Client) func(context.Context, *mcp.CallToolRequest, setTransactionCategoryInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input setTransactionCategoryInput) (*mcp.CallToolResult, any, error) {
		if err := client.SetTransactionCategory(ctx, input.TransactionID, input.CategoryID); err != nil {
			return nil, nil, fmt.Errorf("failed to set category: %w", err)
		}
		if input.CategoryID == "" {
			return textResult(fmt.Sprintf("Transaction %s de-categorised.", input.TransactionID)), nil, nil
		}
		return textResult(fmt.Sprintf("Transaction %s category set to %q.", input.TransactionID, input.CategoryID)), nil, nil
	}
}

// --- Tool: list_tags ---

type listTagsInput struct{}

func listTags(client *up.Client) func(context.Context, *mcp.CallToolRequest, listTagsInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, _ listTagsInput) (*mcp.CallToolResult, any, error) {
		tags, err := client.ListTags(ctx)
		if err != nil {
			return nil, nil, fmt.Errorf("failed to list tags: %w", err)
		}
		if len(tags) == 0 {
			return textResult("No tags found."), nil, nil
		}
		var sb strings.Builder
		for _, t := range tags {
			fmt.Fprintf(&sb, "%s\n", t.ID)
		}
		return textResult(fmt.Sprintf("Tags (%d):\n%s", len(tags), sb.String())), nil, nil
	}
}

// --- Tool: add_tags ---

type addTagsInput struct {
	TransactionID string   `json:"transaction_id" jsonschema:"required,the Up transaction ID"`
	Tags          []string `json:"tags"           jsonschema:"required,list of tag labels to add (max 6 total per transaction)"`
}

func addTags(client *up.Client) func(context.Context, *mcp.CallToolRequest, addTagsInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input addTagsInput) (*mcp.CallToolResult, any, error) {
		if len(input.Tags) == 0 {
			return nil, nil, fmt.Errorf("tags must not be empty")
		}
		if err := client.AddTagsToTransaction(ctx, input.TransactionID, input.Tags); err != nil {
			return nil, nil, fmt.Errorf("failed to add tags: %w", err)
		}
		return textResult(fmt.Sprintf("Added tags %v to transaction %s.", input.Tags, input.TransactionID)), nil, nil
	}
}

// --- Tool: remove_tags ---

type removeTagsInput struct {
	TransactionID string   `json:"transaction_id" jsonschema:"required,the Up transaction ID"`
	Tags          []string `json:"tags"           jsonschema:"required,list of tag labels to remove"`
}

func removeTags(client *up.Client) func(context.Context, *mcp.CallToolRequest, removeTagsInput) (*mcp.CallToolResult, any, error) {
	return func(ctx context.Context, _ *mcp.CallToolRequest, input removeTagsInput) (*mcp.CallToolResult, any, error) {
		if len(input.Tags) == 0 {
			return nil, nil, fmt.Errorf("tags must not be empty")
		}
		if err := client.RemoveTagsFromTransaction(ctx, input.TransactionID, input.Tags); err != nil {
			return nil, nil, fmt.Errorf("failed to remove tags: %w", err)
		}
		return textResult(fmt.Sprintf("Removed tags %v from transaction %s.", input.Tags, input.TransactionID)), nil, nil
	}
}
