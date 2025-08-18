package persona

import (
	_ "embed"
	"fmt"
	"math/rand"

	"github.com/sat8bit/kaigi/configs"
	"gopkg.in/yaml.v3"
)

func NewPool() (*Pool, error) {
	var p Pool
	err := yaml.Unmarshal(configs.Personas, &p)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal embedded Personas: %w", err)
	}
	return &p, nil
}

type Pool struct {
	// Personas は、読み込まれた Persona のスライスです。
	Personas []*Persona `yaml:"personas"`
}

func (p *Pool) GetAll() []*Persona {
	if p == nil {
		return nil
	}
	return p.Personas
}

func (p *Pool) GetByPersonaId(personaId string) (*Persona, error) {
	for _, persona := range p.Personas {
		if persona.PersonaId == personaId {
			return persona, nil
		}
	}
	return nil, fmt.Errorf("persona with id '%s' not found", personaId)
}

func (p *Pool) GetRandomN(n int) ([]*Persona, error) {
	if p == nil || len(p.Personas) == 0 {
		return nil, fmt.Errorf("no Personas available")
	}
	if n <= 0 || n > len(p.Personas) {
		n = len(p.Personas)
	}

	// ランダムに選ぶためのスライスを作成
	selected := make([]*Persona, 0, n)
	usedIndices := make(map[int]struct{})

	for len(selected) < n {
		index := rand.Intn(len(p.Personas))
		if _, exists := usedIndices[index]; !exists {
			selected = append(selected, p.Personas[index])
			usedIndices[index] = struct{}{}
		}
	}

	return selected, nil
}
