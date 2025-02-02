package duckduckgo

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/henomis/restclientgo"
)

const (
	defaultTimeoutInSeconds = 60
)

type Tool struct {
	maxResults uint
	userAgent  string
	restClient *restclientgo.RestClient
}

type Input struct {
	Query string `json:"query" jsonschema:"description=the query to search for"`
}

type Output struct {
	Error   string   `json:"error,omitempty"`
	Results []result `json:"results,omitempty"`
}

type FnPrototype func(Input) Output

func New() *Tool {
	t := &Tool{
		maxResults: 1,
	}

	restClient := restclientgo.New("https://html.duckduckgo.com").
		WithRequestModifier(
			func(r *http.Request) *http.Request {
				r.Header.Add("User-Agent", t.userAgent)
				return r
			},
		)

	t.restClient = restClient
	return t
}

func (t *Tool) WithUserAgent(userAgent string) *Tool {
	t.userAgent = userAgent
	return t
}

func (t *Tool) WithMaxResults(maxResults uint) *Tool {
	t.maxResults = maxResults
	return t
}

func (t *Tool) Name() string {
	return "duckduckgo"
}

func (t *Tool) Description() string {
	return "A tool that uses the DuckDuckGo internet search engine for a query."
}

func (t *Tool) Fn() any {
	return t.fn
}

func (t *Tool) fn(i Input) Output {
	ctx, cancel := context.WithTimeout(context.Background(), defaultTimeoutInSeconds*time.Second)
	defer cancel()

	req := &request{Query: i.Query}
	res := &response{MaxResults: t.maxResults}

	err := t.restClient.Get(ctx, req, res)
	if err != nil {
		return Output{Error: fmt.Sprintf("failed to search DuckDuckGo: %v", err)}
	}

	return Output{Results: res.Results}
}
