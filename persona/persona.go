package persona

// Role は、ペルソナの役割を定義する型です。
type Role string

const (
	RoleRegular Role = "regular"
	RoleGuest   Role = "guest"
)

// Persona は、Cha の人格（ペルソナ）を定義します。
// この情報は、LLMに渡すプロンプトのベースとなります。
type Persona struct {
	PersonaId       string   `yaml:"personaId"`
	DisplayName     string   `yaml:"displayName"`
	Role            Role     `yaml:"role,omitempty"`
	Gender          string   `yaml:"gender"`
	Tagline         string   `yaml:"tagline"`
	StyleTag        string   `yaml:"styleTag"`
	Catchphrases    []string `yaml:"catchphrases"`
	DefaultMaxChars int      `yaml:"defaultMaxChars"`
	SpeakProb       float64  `yaml:"speakProb"`
	MinGapSeconds   int      `yaml:"minGapSeconds"`
}
