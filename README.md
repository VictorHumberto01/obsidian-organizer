# Obsidian Organizer

## Overview

Obsidian Organizer is a command-line tool, written in Go, that scans an Obsidian vault for notes marked as `Quick Notes` and uses a language model to reclassify each one into an existing category, or into a new category created on the fly. It is meant to run periodically (or on demand) against a vault to keep loose, untriaged notes from piling up.

The tool reads the list of valid categories directly from the vault's `Categories/` folder, builds a classification prompt from that list, and delegates the actual classification to a pluggable LLM backend. Two backends are supported:

- Apple Intelligence, through a local `apfel` command, for macOS.
- Ollama, for any computer running a local or remote Ollama server.

## Objective

The goal of this project is to automate the triage step of a note-taking workflow: instead of manually deciding which category a quickly captured note belongs to, the tool inspects the note's title and body, asks an LLM to pick the best matching category from the vault, and rewrites the note's frontmatter tag accordingly. When no existing category is a good fit, the tool can propose a brand new category, ask for confirmation, and create the corresponding file in `Categories/`.

Design goals:

- Keep the LLM backend swappable behind a single `Provider` interface (`llm.Provider`), so new backends can be added without touching the main workflow.
- Derive the category list from the vault itself, so the tool stays in sync with however the vault is actually organized.
- Support an interactive mode (confirm/rename new categories) and a fast mode (skip interaction, keep unmatched notes as `Quick Notes`) for unattended runs.
- Keep a permanent log (`organizer.log`) of every triage decision.

## Requirements

- Go 1.26 or later (see `go.mod`).
- An Obsidian vault where notes to be triaged start with YAML frontmatter containing a `[[Quick Notes]]` tag, and a `Categories/` folder containing one `.md` file per valid category.
- For `-mode=mac`: macOS with Apple Intelligence enabled and the `apfel` executable available on `PATH`.
- For `-mode=ollama`: an Ollama server, local or remote, with the desired model already pulled (for example, `ollama pull phi3:mini`).

## Usage

Build or run the tool from the project root, pointing `-dir` at the root of the Obsidian vault:

```
go run . -mode=mac -dir=/path/to/vault
go run . -mode=ollama -dir=/path/to/vault
```

Available flags:

| Flag | Default | Description |
| --- | --- | --- |
| `-mode` | `mock` | AI backend to use: `mac` (Apple Intelligence via `apfel`) or `ollama`. There is no working `mock` backend yet, so `-mode` must be set explicitly to `mac` or `ollama`. |
| `-dir` | `.` | Root directory of the Obsidian vault to scan. |
| `-fast` | `false` | Skips interactive prompts. Notes for which no existing category matches are kept as `Quick Notes` instead of prompting to create a new one. |
| `-ollama-url` | `http://localhost:11434/api/generate` | URL of the Ollama server's generate endpoint. Only used with `-mode=ollama`. |
| `-ollama-model` | `phi3:mini` | Name of the Ollama model to use. Only used with `-mode=ollama`. |

Examples:

```
# Local classification on macOS using Apple Intelligence
go run . -mode=mac -dir=~/Documents/MyVault

# Classification using a local Ollama instance
go run . -mode=ollama -dir=~/Documents/MyVault

# Classification using a remote Ollama server and a different model
go run . -mode=ollama -dir=~/Documents/MyVault -ollama-url=http://my-server.local:11434/api/generate -ollama-model=llama3.1:8b

# Unattended run: unmatched notes stay as Quick Notes instead of prompting
go run . -mode=ollama -dir=~/Documents/MyVault -fast
```

Notes on vault layout and behavior:

- Only top-level `.md` files in `-dir` are scanned; subdirectories, `Attachments`, `Categories`, `Templates`, `Excalidraw`, `organizer.log` and `organizer_rules.md` are ignored.
- A note is only processed if its YAML frontmatter contains the literal tag `[[Quick Notes]]`.
- If a vault-level `organizer_rules.md` file exists at the root of `-dir`, its contents are appended to the classification prompt as vault-specific rules.
- When the model suggests a category that does not yet exist, the tool creates a new `.md` file for it under `Categories/`, unless `-fast` is set, in which case the note is left untouched.
- Every decision (kept as `Quick Notes` or promoted to a category) is appended to `organizer.log` in the vault root, in addition to being printed to the console.

## Writing Effective Rules (organizer_rules.md)

`organizer_rules.md`, if present at the root of `-dir`, is not summarized or interpreted by the tool: its raw contents are appended, verbatim, to the classification prompt that is sent to the model for every single note. Understanding exactly how it fits into that prompt is the key to writing rules that work.

The full prompt sent to the model is assembled in this order:

```
Você é um classificador de arquivos inflexível e cirúrgico. Escolha a melhor categoria da
lista abaixo baseando-se no TÍTULO e no CONTEÚDO da nota.

[ CATEGORIAS VÁLIDAS ]: <categories found in Categories/, comma-separated>

REGRAS GERAIS:
1. Não seja preguiçoso. EVITE usar "Quick Notes" a menos que a nota seja literalmente
   lixo, um rascunho sem sentido ou uma lista de compras banal.
2. NUNCA invente categorias ou traduza os nomes. A saída deve ser EXATAMENTE uma das
   palavras dentro da lista de Categorias Válidas.
3. Responda APENAS o nome escolhido. Sem explicações, sem aspas, sem pontos finais.

REGRAS ESPECÍFICAS DESTE COFRE:
<contents of organizer_rules.md, inserted as-is>

Texto: '<note title and body>'
```

Everything below `REGRAS ESPECÍFICAS DESTE COFRE:` is entirely up to you. In practice, that means `organizer_rules.md` is your opportunity to encode the vault-specific judgment calls that a generic classifier cannot know on its own: which category wins when a note touches two topics, which keywords should always map to a given category, and which edge cases deserve special handling.

Guidelines for writing rules that actually change model behavior:

- Write one instruction per line, as short, direct, imperative statements. Small local models (like `phi3:mini`) follow a flat list of concrete rules far more reliably than a few dense paragraphs of prose.
- Reference categories using their exact name as it appears in `Categories/` (the `.md` filename without the extension), with matching capitalization. The model must output that exact string, and any mismatch will be treated as a brand-new category suggestion instead of a match.
- Prefer concrete triggers over abstract intent. Instead of "classify personal finance notes correctly", write something like "Notes that mention expenses, invoices, budgets or investments must be Finance, even if short."
- Resolve foreseeable ambiguity explicitly instead of leaving it to chance. If two categories could both plausibly apply, state which one takes priority: "If a note mentions both a recipe and a social event, prefer Cooking over Social."
- Call out exceptions to the general rules explicitly, especially exceptions to rule 1 above (the bias against `Quick Notes`). For example: "Even one-line notes about medication or symptoms must be Health, never Quick Notes."
- Do not ask the model to explain its reasoning, add punctuation, use markdown, or wrap the answer in quotes. That directly conflicts with general rule 3 ("answer only the chosen name") and will break the tool's parsing, since the raw response is used as the category name almost as-is (see `sanitizeCategory` in `main.go`).
- Do not tell the model to invent or rename categories; that conflicts with general rule 2 and with how the tool decides whether a category already exists.
- Keep the language of your rules consistent with the language of your notes and category names. The base persona above is in Portuguese; if your vault and categories are in another language, mirror that language in `organizer_rules.md` so keyword-style rules actually match what appears in your notes.
- Keep the file short and specific. It is resent in full on every classification call, so a bloated rules file adds latency (particularly noticeable with `-mode=ollama` on small models) and can dilute a small model's attention, making it less likely to follow any single rule.
- Treat the file as living documentation: after a run, check the console output and `organizer.log` for notes that were mis-classified or unexpectedly kept as `Quick Notes`, then add or adjust a rule to cover that specific case.

Example `organizer_rules.md`, assuming a vault with `Health.md`, `Finance.md`, `Cooking.md` and `Work.md` under `Categories/`:

```
1. Notas que mencionem consultas médicas, exames, sintomas ou medicamentos devem ser
   Health, mesmo que curtas.
2. Notas sobre gastos, faturas, orçamento ou investimentos devem ser Finance.
3. Notas sobre receitas, ingredientes ou técnicas de cozinha devem ser Cooking, mesmo
   que também mencionem um evento social.
4. Anotações de reuniões de trabalho devem ser Work, mesmo que tenham poucas linhas.
5. Em caso de empate entre Health e Work (ex: consulta médica marcada durante o
   expediente), prefira Health.
```

When iterating on rules, `-mode=ollama` is the faster feedback loop: it runs locally, has no macOS-specific dependency, and lets you swap `-ollama-model` freely (for example between `phi3:mini` and a larger model such as `qwen2.5:7b` or `llama3.1:8b`) to see how much rule adherence varies by model size before settling on a final configuration for everyday use.

## Building

Fetch dependencies (if any are added in the future) and build a binary:

```
go build -o obsidian-organizer .
```

Run the resulting binary directly:

```
./obsidian-organizer -mode=mac -dir=/path/to/vault
```

To install the binary into `$GOPATH/bin` (or `$GOBIN`):

```
go install .
```

To verify the project compiles and passes static checks:

```
go build ./...
go vet ./...
```
