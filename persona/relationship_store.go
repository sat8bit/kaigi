package persona

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// RelationshipStore は、ペルソナ間の関係性データの読み書きを管理します。
type RelationshipStore struct {
	dataDir string
}

// NewRelationshipStore は、新しい RelationshipStore を生成します。
func NewRelationshipStore(dataDir string) *RelationshipStore {
	return &RelationshipStore{dataDir: dataDir}
}

// LoadForPersona は、指定されたペルソナの Relationships マップをファイルから読み込み、設定します。
func (s *RelationshipStore) LoadForPersona(p *Persona) error {
	p.Relationships = make(map[string]*Relationship) // 初期化
	relPath := filepath.Join(s.dataDir, "relationships", p.PersonaId+".yaml")

	if _, err := os.Stat(relPath); os.IsNotExist(err) {
		return nil // ファイルがなければ何もしない（エラーではない）
	}

	data, err := os.ReadFile(relPath)
	if err != nil {
		return fmt.Errorf("failed to read relationship file %s: %w", relPath, err)
	}

	var rels []*Relationship
	if err := yaml.Unmarshal(data, &rels); err != nil {
		return fmt.Errorf("failed to unmarshal relationship file %s: %w", relPath, err)
	}

	for _, r := range rels {
		p.Relationships[r.TargetPersonaId] = r
	}
	return nil
}

// SaveForPersona は、指定されたペルソナの現在の関係性をファイルに保存します。
func (s *RelationshipStore) SaveForPersona(p *Persona) error {
	if p.Relationships == nil || len(p.Relationships) == 0 {
		return nil // 保存すべきデータがない
	}

	// マップをスライスに変換（YAML化のため）
	rels := make([]*Relationship, 0, len(p.Relationships))
	for _, r := range p.Relationships {
		rels = append(rels, r)
	}

	data, err := yaml.Marshal(rels)
	if err != nil {
		return fmt.Errorf("failed to marshal relationships for %s: %w", p.PersonaId, err)
	}

	relDir := filepath.Join(s.dataDir, "relationships")
	if err := os.MkdirAll(relDir, 0755); err != nil {
		return fmt.Errorf("failed to create relationship directory %s: %w", relDir, err)
	}
	
	relPath := filepath.Join(relDir, p.PersonaId+".yaml")
	if err := os.WriteFile(relPath, data, 0644); err != nil {
		return fmt.Errorf("failed to write relationship file %s: %w", relPath, err)
	}

	return nil
}
