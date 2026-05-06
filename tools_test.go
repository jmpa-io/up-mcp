package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sync/atomic"
	"testing"

	up "github.com/jmpa-io/up-go"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// --- mock round tripper ---

type mockRoundTripper struct {
	fn func(*http.Request) *http.Response
}

func (m *mockRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return m.fn(req), nil
}

// okResp returns a 200 response with a JSON body.
func okResp(t *testing.T, v any) *http.Response {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("okResp: marshal: %v", err)
	}
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewReader(b)),
	}
}

// noContentResp returns a 204 No Content response (used for PATCH/DELETE).
func noContentResp() *http.Response {
	return &http.Response{
		StatusCode: http.StatusNoContent,
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewReader(nil)),
	}
}

// pingJSON is the raw JSON for a successful ping response — used during
// newTestClient init without needing a *testing.T.
var pingJSON = []byte(`{"meta":{"id":"test-ping-id","statusEmoji":"⚡️"}}`)

// pingResp returns a valid ping response for the New() init call.
func pingResp() *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(bytes.NewReader(pingJSON)),
	}
}

// newTestClient creates an Up client whose HTTP transport answers the first
// request (the init ping from New()) with a successful ping, then delegates
// all subsequent requests to fn.
func newTestClient(t *testing.T, fn func(*http.Request) *http.Response) *up.Client {
	t.Helper()
	var count atomic.Int32
	rt := &mockRoundTripper{fn: func(req *http.Request) *http.Response {
		if count.Add(1) == 1 {
			return pingResp()
		}
		return fn(req)
	}}
	c, err := up.New(context.Background(), "up:test:xxxx",
		up.WithHttpClient(&http.Client{Transport: rt}),
		up.WithLogLevel(100), // suppress all logs during tests
	)
	if err != nil {
		t.Fatalf("newTestClient: %v", err)
	}
	return c
}

// callTool is a generic helper that calls a registered tool handler directly
// without going through the MCP wire protocol.
func callTool[I any](t *testing.T, handler func(context.Context, *mcp.CallToolRequest, I) (*mcp.CallToolResult, any, error), input I) (*mcp.CallToolResult, error) {
	t.Helper()
	result, _, err := handler(context.Background(), &mcp.CallToolRequest{}, input)
	return result, err
}

// resultText extracts the text from the first content item of a tool result.
func resultText(t *testing.T, r *mcp.CallToolResult) string {
	t.Helper()
	if r == nil || len(r.Content) == 0 {
		t.Fatal("resultText: nil or empty result")
	}
	tc, ok := r.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("resultText: first content item is not *mcp.TextContent")
	}
	return tc.Text
}

// --- Tests ---

func Test_pingTool(t *testing.T) {
	client := newTestClient(t, func(req *http.Request) *http.Response {
		return pingResp()
	})
	result, err := callTool(t, pingTool(client), pingInput{})
	if err != nil {
		t.Fatalf("pingTool() error: %v", err)
	}
	text := resultText(t, result)
	if text == "" {
		t.Error("pingTool() returned empty text")
	}
}

func Test_listAccounts(t *testing.T) {
	accounts := map[string]any{
		"data": []any{
			map[string]any{
				"type": "accounts",
				"id":   "abc123",
				"attributes": map[string]any{
					"displayName":   "Spending",
					"accountType":   "TRANSACTIONAL",
					"ownershipType": "INDIVIDUAL",
					"balance": map[string]any{
						"currencyCode":     "AUD",
						"value":            "1200.00",
						"valueInBaseUnits": 120000,
					},
					"createdAt": "2024-01-01T00:00:00+10:00",
				},
				"relationships": map[string]any{},
			},
		},
		"links": map[string]any{"next": nil, "prev": nil},
	}

	client := newTestClient(t, func(req *http.Request) *http.Response {
		return okResp(t, accounts)
	})

	result, err := callTool(t, listAccounts(client), listAccountsInput{})
	if err != nil {
		t.Fatalf("listAccounts() error: %v", err)
	}
	text := resultText(t, result)
	if text == "" {
		t.Error("listAccounts() returned empty text")
	}
}

func Test_listTransactions(t *testing.T) {
	txns := map[string]any{
		"data": []any{
			map[string]any{
				"type": "transactions",
				"id":   "txn-001",
				"attributes": map[string]any{
					"description": "Grill'd",
					"amount": map[string]any{
						"currencyCode":     "AUD",
						"value":            "-25.00",
						"valueInBaseUnits": -2500,
					},
					"status":    "SETTLED",
					"createdAt": "2026-05-01T18:30:00+10:00",
				},
				"relationships": map[string]any{},
			},
		},
		"links": map[string]any{"next": nil, "prev": nil},
	}

	client := newTestClient(t, func(req *http.Request) *http.Response {
		return okResp(t, txns)
	})

	result, err := callTool(t, listTransactions(client), listTransactionsInput{
		Since: "2026-05-01T00:00:00+10:00",
	})
	if err != nil {
		t.Fatalf("listTransactions() error: %v", err)
	}
	text := resultText(t, result)
	if text == "" {
		t.Error("listTransactions() returned empty text")
	}
}

func Test_listTransactions_invalid_since(t *testing.T) {
	client := newTestClient(t, func(req *http.Request) *http.Response {
		return pingResp()
	})
	_, err := callTool(t, listTransactions(client), listTransactionsInput{
		Since: "not-a-date",
	})
	if err == nil {
		t.Error("listTransactions() expected error for invalid since, got nil")
	}
}

func Test_listTransactionsByAccount(t *testing.T) {
	txns := map[string]any{
		"data":  []any{},
		"links": map[string]any{"next": nil, "prev": nil},
	}
	client := newTestClient(t, func(req *http.Request) *http.Response {
		return okResp(t, txns)
	})
	result, err := callTool(t, listTransactionsByAccount(client), listTransactionsByAccountInput{
		AccountID: "acct-001",
	})
	if err != nil {
		t.Fatalf("listTransactionsByAccount() error: %v", err)
	}
	if result == nil {
		t.Error("listTransactionsByAccount() returned nil result")
	}
}

func Test_getTransaction(t *testing.T) {
	txn := map[string]any{
		"data": map[string]any{
			"type": "transactions",
			"id":   "txn-abc",
			"attributes": map[string]any{
				"description": "Woolworths",
				"amount": map[string]any{
					"currencyCode":     "AUD",
					"value":            "-87.50",
					"valueInBaseUnits": -8750,
				},
				"status":    "SETTLED",
				"createdAt": "2026-05-01T09:00:00+10:00",
			},
			"relationships": map[string]any{},
		},
	}
	client := newTestClient(t, func(req *http.Request) *http.Response {
		return okResp(t, txn)
	})
	result, err := callTool(t, getTransaction(client), getTransactionInput{TransactionID: "txn-abc"})
	if err != nil {
		t.Fatalf("getTransaction() error: %v", err)
	}
	text := resultText(t, result)
	if text == "" {
		t.Error("getTransaction() returned empty text")
	}
}

func Test_listCategories(t *testing.T) {
	cats := map[string]any{
		"data": []any{
			map[string]any{
				"type": "categories",
				"id":   "restaurants-and-cafes",
				"attributes": map[string]any{
					"name": "Restaurants & Cafes",
				},
				"relationships": map[string]any{
					"parent": map[string]any{"data": map[string]any{"type": "categories", "id": "good-life"}},
					"children": map[string]any{"data": []any{}},
				},
			},
		},
		"links": map[string]any{"next": nil, "prev": nil},
	}
	client := newTestClient(t, func(req *http.Request) *http.Response {
		return okResp(t, cats)
	})
	result, err := callTool(t, listCategories(client), listCategoriesInput{})
	if err != nil {
		t.Fatalf("listCategories() error: %v", err)
	}
	if result == nil {
		t.Error("listCategories() returned nil result")
	}
}

func Test_setTransactionCategory(t *testing.T) {
	client := newTestClient(t, func(req *http.Request) *http.Response {
		return noContentResp()
	})
	result, err := callTool(t, setTransactionCategory(client), setTransactionCategoryInput{
		TransactionID: "txn-abc",
		CategoryID:    "restaurants-and-cafes",
	})
	if err != nil {
		t.Fatalf("setTransactionCategory() error: %v", err)
	}
	text := resultText(t, result)
	if text == "" {
		t.Error("setTransactionCategory() returned empty text")
	}
}

func Test_setTransactionCategory_decategorise(t *testing.T) {
	client := newTestClient(t, func(req *http.Request) *http.Response {
		return noContentResp()
	})
	result, err := callTool(t, setTransactionCategory(client), setTransactionCategoryInput{
		TransactionID: "txn-abc",
		CategoryID:    "", // empty = de-categorise
	})
	if err != nil {
		t.Fatalf("setTransactionCategory() de-categorise error: %v", err)
	}
	text := resultText(t, result)
	if text == "" {
		t.Error("setTransactionCategory() de-categorise returned empty text")
	}
}

func Test_listTags(t *testing.T) {
	tags := map[string]any{
		"data": []any{
			map[string]any{"type": "tags", "id": "Dinner", "relationships": map[string]any{}},
			map[string]any{"type": "tags", "id": "Date night", "relationships": map[string]any{}},
		},
		"links": map[string]any{"next": nil, "prev": nil},
	}
	client := newTestClient(t, func(req *http.Request) *http.Response {
		return okResp(t, tags)
	})
	result, err := callTool(t, listTags(client), listTagsInput{})
	if err != nil {
		t.Fatalf("listTags() error: %v", err)
	}
	text := resultText(t, result)
	if text == "" {
		t.Error("listTags() returned empty text")
	}
}

func Test_addTags(t *testing.T) {
	client := newTestClient(t, func(req *http.Request) *http.Response {
		return noContentResp()
	})
	result, err := callTool(t, addTags(client), addTagsInput{
		TransactionID: "txn-abc",
		Tags:          []string{"Dinner", "Date night"},
	})
	if err != nil {
		t.Fatalf("addTags() error: %v", err)
	}
	text := resultText(t, result)
	if text == "" {
		t.Error("addTags() returned empty text")
	}
}

func Test_addTags_empty_tags(t *testing.T) {
	client := newTestClient(t, func(req *http.Request) *http.Response {
		return noContentResp()
	})
	_, err := callTool(t, addTags(client), addTagsInput{
		TransactionID: "txn-abc",
		Tags:          []string{},
	})
	if err == nil {
		t.Error("addTags() expected error for empty tags, got nil")
	}
}

func Test_removeTags(t *testing.T) {
	client := newTestClient(t, func(req *http.Request) *http.Response {
		return noContentResp()
	})
	result, err := callTool(t, removeTags(client), removeTagsInput{
		TransactionID: "txn-abc",
		Tags:          []string{"Dinner"},
	})
	if err != nil {
		t.Fatalf("removeTags() error: %v", err)
	}
	text := resultText(t, result)
	if text == "" {
		t.Error("removeTags() returned empty text")
	}
}

func Test_removeTags_empty_tags(t *testing.T) {
	client := newTestClient(t, func(req *http.Request) *http.Response {
		return noContentResp()
	})
	_, err := callTool(t, removeTags(client), removeTagsInput{
		TransactionID: "txn-abc",
		Tags:          []string{},
	})
	if err == nil {
		t.Error("removeTags() expected error for empty tags, got nil")
	}
}
