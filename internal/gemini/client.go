// Package gemini provides a Vertex AI Gemini client for SQL generation.
package gemini

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/nlink-jp/nlk/backoff"
	"github.com/nlink-jp/nlk/strip"
	"google.golang.org/genai"
)

const maxRetries = 3

// Client wraps the Gemini API client.
type Client struct {
	inner *genai.Client
	model string
}

// New creates a new Gemini client for Vertex AI.
func New(ctx context.Context, project, location, model string) (*Client, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		Backend:  genai.BackendVertexAI,
		Project:  project,
		Location: location,
	})
	if err != nil {
		return nil, fmt.Errorf("create genai client: %w", err)
	}
	return &Client{inner: client, model: model}, nil
}

// GenerateSQL sends a prompt to Gemini and returns the generated text.
func (c *Client) GenerateSQL(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	contents := []*genai.Content{
		genai.NewContentFromText(userPrompt, genai.Role("user")),
	}
	config := &genai.GenerateContentConfig{
		Temperature: ptrFloat32(0.1),
	}
	if systemPrompt != "" {
		config.SystemInstruction = genai.NewContentFromText(systemPrompt, genai.Role(""))
	}

	bo := backoff.New(
		backoff.WithBase(2*time.Second),
		backoff.WithMax(30*time.Second),
	)

	for attempt := range maxRetries + 1 {
		resp, err := c.inner.Models.GenerateContent(ctx, c.model, contents, config)
		if err == nil {
			return c.extractText(resp), nil
		}
		if !isRetryable(err) || attempt == maxRetries {
			return "", fmt.Errorf("gemini API: %w", err)
		}
		time.Sleep(bo.Duration(attempt))
	}

	return "", fmt.Errorf("gemini API: max retries exceeded")
}

// GenerateWithHistory sends a multi-turn conversation to Gemini.
func (c *Client) GenerateWithHistory(ctx context.Context, systemPrompt string, history []*genai.Content) (string, error) {
	config := &genai.GenerateContentConfig{
		Temperature: ptrFloat32(0.1),
	}
	if systemPrompt != "" {
		config.SystemInstruction = genai.NewContentFromText(systemPrompt, genai.Role(""))
	}

	bo := backoff.New(
		backoff.WithBase(2*time.Second),
		backoff.WithMax(30*time.Second),
	)

	for attempt := range maxRetries + 1 {
		resp, err := c.inner.Models.GenerateContent(ctx, c.model, history, config)
		if err == nil {
			return c.extractText(resp), nil
		}
		if !isRetryable(err) || attempt == maxRetries {
			return "", fmt.Errorf("gemini API: %w", err)
		}
		time.Sleep(bo.Duration(attempt))
	}

	return "", fmt.Errorf("gemini API: max retries exceeded")
}

func (c *Client) extractText(resp *genai.GenerateContentResponse) string {
	if resp == nil || len(resp.Candidates) == 0 || resp.Candidates[0].Content == nil {
		return ""
	}
	var parts []string
	for _, p := range resp.Candidates[0].Content.Parts {
		if p.Text != "" {
			parts = append(parts, p.Text)
		}
	}
	return strip.ThinkTags(strings.Join(parts, ""))
}

func isRetryable(err error) bool {
	s := strings.ToLower(err.Error())
	for _, kw := range []string{"429", "rate_limit", "quota", "resource_exhausted", "unavailable"} {
		if strings.Contains(s, kw) {
			return true
		}
	}
	return false
}

func ptrFloat32(f float32) *float32 { return &f }
