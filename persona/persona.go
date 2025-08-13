package persona

// Persona は、Cha の人格（ペルソナ）を定義します。
// この情報は、LLMに渡すプロンプトのベースとなります。
type Persona struct {
	PersonaId       string   `yaml:"personaId"`
	DisplayName     string   `yaml:"displayName"`
	Gender          string   `yaml:"gender"`
	Tagline         string   `yaml:"tagline"`
	StyleTag        string   `yaml:"styleTag"`
	Catchphrases    []string `yaml:"catchphrases"`
	DefaultMaxChars int      `yaml:"defaultMaxChars"`
	SpeakProb       float64  `yaml:"speakProb"`
	MinGapSeconds   int      `yaml:"minGapSeconds"`
}
