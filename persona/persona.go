package persona

// Role は、ペルソナの役割を定義する型です。
type Role string

const (
	RoleRegular Role = "regular"
	RoleGuest   Role = "guest"
)

// Relationship は、あるペルソナから見た別のペルソナへの関係性を示します。
// このデータは data/relationships/ ディレクトリのYAMLファイルから読み書きされます。
type Relationship struct {
	TargetPersonaId string `yaml:"targetPersonaId"`
	Affinity        int    `yaml:"affinity"`
	Impression      string `yaml:"impression"`
}

// Persona は、Cha の人格（ペルソナ）を定義します。
// この情報は、LLMに渡すプロンプトのベースとなります。
type Persona struct {
	// --- 静的データ (configs/personas.yaml から) ---
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

	// --- 動的データ (data/relationships/ から) ---
	// 他のペルソナへの関係性を保持するマップ
	// キー: 相手のペルソナの PersonaId
	Relationships map[string]*Relationship `yaml:"-"` // このフィールドはYAMLの直接の対象外
}
