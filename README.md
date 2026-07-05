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
