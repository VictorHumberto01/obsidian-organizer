package llm

import (
	"fmt"
	"os/exec"
	"strings"
)

type ApfelProvider struct {
	Persona string
}

func (p *ApfelProvider) Categorize(text string) (string, error) {
	prompt := fmt.Sprintf("%s\nTexto: '%s'", p.Persona, text)
	

	cmd := exec.Command("apfel", prompt)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("falha ao rodar apfel: %w", err)
	}
	

	return strings.TrimSpace(string(out)), nil
}
