package lsp

// LanguageConfig holds the runtime configurations needed to natively parse
// and syntactically analyze a specific programming language.
type LanguageConfig struct {
	LanguageName string
	Extensions   []string
	LSPBinary    string
	LSPArgs      []string
}

// Registry maps file extensions (e.g., ".py") to their language configuration.
type Registry struct {
	Configs map[string]*LanguageConfig
}

// NewRegistry initializes the universal Language Config mapping.
func NewRegistry() *Registry {
	r := &Registry{
		Configs: make(map[string]*LanguageConfig),
	}
	r.registerDefaults()
	return r
}

func (r *Registry) registerDefaults() {
	// Golang
	r.addConfig(&LanguageConfig{
		LanguageName: "go",
		Extensions:   []string{".go"},
		LSPBinary:    "gopls",
		LSPArgs:      []string{},
	})

	// Python
	r.addConfig(&LanguageConfig{
		LanguageName: "python",
		Extensions:   []string{".py"},
		LSPBinary:    "pyright-langserver",
		LSPArgs:      []string{"--stdio"},
	})

	// JavaScript / TypeScript
	jsTs := &LanguageConfig{
		LanguageName: "typescript",
		Extensions:   []string{".js", ".jsx", ".ts", ".tsx"},
		LSPBinary:    "typescript-language-server",
		LSPArgs:      []string{"--stdio"},
	}
	r.addConfig(jsTs)

	// C / C++
	cCpp := &LanguageConfig{
		LanguageName: "c",
		Extensions:   []string{".c", ".cpp", ".h", ".hpp"},
		LSPBinary:    "clangd",
		LSPArgs:      []string{}, // clangd natively listens on stdio
	}
	r.addConfig(cCpp)

	// Rust
	r.addConfig(&LanguageConfig{
		LanguageName: "rust",
		Extensions:   []string{".rs"},
		LSPBinary:    "rust-analyzer",
		LSPArgs:      []string{}, // native stdio
	})

	// Swift
	r.addConfig(&LanguageConfig{
		LanguageName: "swift",
		Extensions:   []string{".swift"},
		LSPBinary:    "sourcekit-lsp",
		LSPArgs:      []string{},
	})
}

func (r *Registry) addConfig(cfg *LanguageConfig) {
	for _, ext := range cfg.Extensions {
		r.Configs[ext] = cfg
	}
}

// GetConfig returns the associated LanguageConfig for a given file extension.
func (r *Registry) GetConfig(extension string) *LanguageConfig {
	return r.Configs[extension]
}
