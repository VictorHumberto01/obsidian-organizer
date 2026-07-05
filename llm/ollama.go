package llm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type OllamaProvider struct {
	Persona string
	URL     string
	Model   string
}

type ollamaRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
	Stream bool   `json:"stream"`
}

type ollamaResponse struct {
	Response string `json:"response"`
}

func (p *OllamaProvider) Categorize(text string) (string, error) {
	reqBody := ollamaRequest{
		Model:  p.Model,
		Prompt: fmt.Sprintf("%s\nTexto: '%s'", p.Persona, text),
		Stream: false,
	}

	jsonBody, _ := json.Marshal(reqBody)

	resp, err := http.Post(p.URL, "application/json", bytes.NewBuffer(jsonBody))
	if err != nil {
		return "", fmt.Errorf("erro ao conectar ao servidor Ollama: %w", err)
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	var ollamaResp ollamaResponse
	if err := json.Unmarshal(bodyBytes, &ollamaResp); err != nil {
		return "", fmt.Errorf("erro ao ler resposta JSON: %w", err)
	}

	return strings.TrimSpace(ollamaResp.Response), nil
}
