package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const defaultOllamaTimeout = 60 * time.Second

type OllamaProvider struct {
	Persona string
	URL     string
	Model   string

	Timeout time.Duration

	Client *http.Client
}

type ollamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type ollamaResponse struct {
	Response string `json:"response"`
	Error    string `json:"error,omitempty"`
}

func (p *OllamaProvider) Categorize(text string) (string, error) {
	if p.URL == "" {
		return "", fmt.Errorf("ollama: URL do servidor não configurada")
	}
	if p.Model == "" {
		return "", fmt.Errorf("ollama: modelo não configurado")
	}

	reqBody := ollamaRequest{
		Model:  p.Model,
		Prompt: fmt.Sprintf("%s\nTexto: '%s'", p.Persona, text),
		Stream: false,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("ollama: erro ao codificar requisição: %w", err)
	}

	timeout := p.Timeout
	if timeout <= 0 {
		timeout = defaultOllamaTimeout
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.URL, bytes.NewReader(jsonBody))
	if err != nil {
		return "", fmt.Errorf("ollama: erro ao montar requisição: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := p.Client
	if client == nil {
		client = &http.Client{Timeout: timeout}
	}

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("ollama: erro ao conectar ao servidor (%s): %w", p.URL, err)
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("ollama: erro ao ler resposta: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("ollama: servidor retornou status %d: %s", resp.StatusCode, strings.TrimSpace(string(bodyBytes)))
	}

	var ollamaResp ollamaResponse
	if err := json.Unmarshal(bodyBytes, &ollamaResp); err != nil {
		return "", fmt.Errorf("ollama: erro ao decodificar resposta JSON: %w", err)
	}

	if ollamaResp.Error != "" {
		return "", fmt.Errorf("ollama: erro retornado pelo modelo: %s", ollamaResp.Error)
	}

	result := strings.TrimSpace(ollamaResp.Response)
	if result == "" {
		return "", fmt.Errorf("ollama: resposta vazia do modelo %q", p.Model)
	}

	return result, nil
}
