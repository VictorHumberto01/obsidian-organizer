package main

import (
	"bufio"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"obsidian-organizer/llm"
)

const PersonaTemplate = `Você é um classificador de arquivos inflexível e cirúrgico. Escolha a melhor categoria da lista abaixo baseando-se no TÍTULO e no CONTEÚDO da nota.

[ CATEGORIAS VÁLIDAS ]: %s

REGRAS GERAIS:
1. Não seja preguiçoso. EVITE usar "Quick Notes" a menos que a nota seja literalmente lixo, um rascunho sem sentido ou uma lista de compras banal.
2. NUNCA invente categorias ou traduza os nomes. A saída deve ser EXATAMENTE uma das palavras dentro da lista de Categorias Válidas.
3. Responda APENAS o nome escolhido. Sem explicações, sem aspas, sem pontos finais.`

func main() {
	mode := flag.String("mode", "mock", "Estratégia de IA: mac, ollama ou mock")
	dirFlag := flag.String("dir", ".", "Diretório raiz do Obsidian")
	fastFlag := flag.Bool("fast", false, "Ativa modo rápido (pula interação e mantém como Quick Note)")
	ollamaURLFlag := flag.String("ollama-url", "http://localhost:11434/api/generate", "URL do servidor Ollama (modo -mode=ollama)")
	ollamaModelFlag := flag.String("ollama-model", "phi3:mini", "Modelo a ser usado pelo Ollama (modo -mode=ollama)")
	flag.Parse()

	rootDir := *dirFlag
	categoriesDir := filepath.Join(rootDir, "Categories")

	os.MkdirAll(categoriesDir, 0755)

	// Carrega as categorias base dinamicamente da pasta Categories/
	listaCategorias := carregarCategorias(categoriesDir)
	personaDinamica := fmt.Sprintf(PersonaTemplate, listaCategorias)

	// Injeta as regras específicas se o arquivo organizer_rules.md existir no Vault
	rulesPath := filepath.Join(rootDir, "organizer_rules.md")
	if bytesRules, err := os.ReadFile(rulesPath); err == nil {
		personaDinamica += fmt.Sprintf("\n\nREGRAS ESPECÍFICAS DESTE COFRE:\n%s", string(bytesRules))
		fmt.Println("🧠 Arquivo de regras customizadas (organizer_rules.md) carregado!")
	}

	var ai llm.Provider

	switch *mode {
	case "mac":
		fmt.Printf("🤖 Inicializando IA Local via Apfel (macOS) | Fast Mode: %t\n", *fastFlag)
		ai = &llm.ApfelProvider{Persona: personaDinamica}
	case "ollama":
		fmt.Printf("🖥️ Inicializando IA via Ollama (%s, modelo %s) | Fast Mode: %t\n", *ollamaURLFlag, *ollamaModelFlag, *fastFlag)
		ai = &llm.OllamaProvider{
			Persona: personaDinamica,
			URL:     *ollamaURLFlag,
			Model:   *ollamaModelFlag,
		}
	default:
		log.Fatalf("Modo inválido. Use -mode=mac, -mode=ollama ou -mode=mock")
	}

	leitorTeclado := bufio.NewReader(os.Stdin)

	// Configura o arquivo de Log Permanente
	logPath := filepath.Join(rootDir, "organizer.log")
	logFile, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	var fileLogger *log.Logger
	if err == nil {
		defer logFile.Close()
		fileLogger = log.New(logFile, "TRIAGEM: ", log.Ldate|log.Ltime)
	} else {
		fmt.Printf("⚠️ Aviso: Não foi possível criar arquivo de log em %s\n", logPath)
	}

	entries, err := os.ReadDir(rootDir)
	if err != nil {
		log.Fatalf("Erro ao ler a pasta raiz: %v", err)
	}

	ignoreList := map[string]bool{
		"Attachments":        true,
		"Categories":         true,
		"Templates":          true,
		"Excalidraw":         true,
		"organizer.log":      true,
		"organizer_rules.md": true,
	}

	for _, entry := range entries {
		nome := entry.Name()

		if entry.IsDir() || ignoreList[nome] || !strings.HasSuffix(nome, ".md") {
			continue
		}

		caminho := filepath.Join(rootDir, nome)
		processarNota(caminho, ai, leitorTeclado, categoriesDir, *fastFlag, fileLogger)
	}

	fmt.Println("\n🎉 Varredura concluída!")
}

func carregarCategorias(pastaCategories string) string {
	entries, err := os.ReadDir(pastaCategories)
	if err != nil {
		return "Quick Notes"
	}

	var categorias []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".md") {
			nomeLimpo := strings.TrimSuffix(entry.Name(), ".md")
			categorias = append(categorias, nomeLimpo)
		}
	}

	if len(categorias) == 0 {
		return "Quick Notes"
	}

	return strings.Join(categorias, ", ")
}

func processarNota(caminho string, ai llm.Provider, leitor *bufio.Reader, pastaCategories string, fast bool, fileLogger *log.Logger) {
	bytesFile, err := os.ReadFile(caminho)
	if err != nil {
		return
	}

	conteudo := string(bytesFile)
	if !strings.HasPrefix(conteudo, "---\n") {
		return
	}

	partes := strings.SplitN(conteudo, "---", 3)
	if len(partes) < 3 {
		return
	}

	yamlBlock := partes[1]
	corpoNota := strings.TrimSpace(partes[2])

	// Gatilho: Processa apenas se a nota estiver marcada como [[Quick Notes]]
	if !strings.Contains(yamlBlock, "[[Quick Notes]]") {
		return
	}

	// Fornece contexto máximo à IA (Título + Corpo)
	tituloNota := strings.TrimSuffix(filepath.Base(caminho), ".md")
	textoParaIA := fmt.Sprintf("TÍTULO da nota: %s\nCONTEÚDO da nota: %s", tituloNota, corpoNota)

	categoria, err := ai.Categorize(textoParaIA)
	if err != nil {
		log.Printf("Erro na IA para %s: %v\n", filepath.Base(caminho), err)
		return
	}
	categoriaLimpa := sanitizeCategory(categoria)

	if categoriaLimpa == "QuickNotes" || categoriaLimpa == "Quick Notes" {
		if fast {
			fmt.Printf("⏩ [%s] Mantido como [[Quick Notes]] (Modo Fast)\n", filepath.Base(caminho))
			if fileLogger != nil {
				fileLogger.Printf("Mantido: %s -> Quick Notes\n", filepath.Base(caminho))
			}
			return
		}

		fmt.Printf("\n⚠️ A IA não encontrou nenhuma categoria existente que combine com '%s'.\n", filepath.Base(caminho))

		promptSugestao := fmt.Sprintf(`Ignore a lista de categorias atual. Você precisa criar UMA PALAVRA INÉDITA (Nome Próprio, Capitalizado) para ser a NOVA categoria deste texto. É ESTRITAMENTE PROIBIDO responder "Quick Notes" ou "QuickNotes". Texto: '%s'`, textoParaIA)
		sugestaoIA, _ := ai.Categorize(promptSugestao)
		sugestaoIA = sanitizeCategory(sugestaoIA)

		if sugestaoIA == "QuickNotes" || sugestaoIA == "Quick Notes" || sugestaoIA == "" {
			sugestaoIA = "NovaCategoria"
		}

		fmt.Printf("🤖 Sugestão para criar uma NOVA categoria: [[%s]]\n", sugestaoIA)
		fmt.Print("Aceita? [S/n] ou digite um nome customizado: ")

		input, _ := leitor.ReadString('\n')
		input = strings.TrimSpace(input)

		if strings.ToLower(input) == "n" {
			fmt.Printf("⏩ [%s] Mantido como [[Quick Notes]].\n", filepath.Base(caminho))
			if fileLogger != nil {
				fileLogger.Printf("Mantido: %s -> Quick Notes\n", filepath.Base(caminho))
			}
			return
		}

		if input == "" || strings.ToLower(input) == "s" {
			categoriaLimpa = sugestaoIA
		} else {
			categoriaLimpa = sanitizeCategory(input)
		}

		criarCategoriaFisica(pastaCategories, categoriaLimpa)
	}

	novaTag := fmt.Sprintf("[[%s]]", categoriaLimpa)
	novoYaml := strings.Replace(yamlBlock, "[[Quick Notes]]", novaTag, 1)
	novoConteudo := fmt.Sprintf("---%s---%s", novoYaml, corpoNota)

	err = os.WriteFile(caminho, []byte(novoConteudo), 0644)
	if err != nil {
		log.Printf("Erro ao salvar nota %s: %v\n", filepath.Base(caminho), err)
	} else {
		fmt.Printf("✅ [%s] Categorizado -> %s\n", filepath.Base(caminho), novaTag)
		if fileLogger != nil {
			fileLogger.Printf("Promovido: %s -> %s\n", filepath.Base(caminho), categoriaLimpa)
		}
	}
}

func criarCategoriaFisica(pastaCategories, nomeCategoria string) error {
	caminhoArquivo := filepath.Join(pastaCategories, nomeCategoria+".md")

	if _, err := os.Stat(caminhoArquivo); err == nil {
		return nil
	}

	template := fmt.Sprintf(`---
tags:
  - categories
banner_y: 50.0%%
banner: "[[DashboardBanner.gif]]"
title: "# %s"
---

![[%s.base]]
`, nomeCategoria, nomeCategoria)

	err := os.WriteFile(caminhoArquivo, []byte(template), 0644)
	if err == nil {
		fmt.Printf("📁 Nova categoria criada no Vault: Categories/%s.md\n", nomeCategoria)
	}
	return err
}

func sanitizeCategory(cat string) string {
	cat = strings.TrimSpace(cat)
	cat = strings.ReplaceAll(cat, ".", "")
	cat = strings.ReplaceAll(cat, "\"", "")
	cat = strings.ReplaceAll(cat, "'", "")

	if cat == "" {
		return "Quick Notes"
	}
	return cat
}
