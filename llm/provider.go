package llm

type Provider interface {
	Categorize(text string) (string, error)
}
