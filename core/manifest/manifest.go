package manifest

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type Repo struct {
	Name     string `yaml:"name"`
	Remote   string `yaml:"remote"`
	Baseline string `yaml:"baseline,omitempty"`
	Label    string `yaml:"label,omitempty"`
}

type Manifest struct {
	Project         string `yaml:"project"`
	DefaultBaseline string `yaml:"defaultBaseline"`
	Host            *Repo  `yaml:"host,omitempty"`
	Repos           []Repo `yaml:"repos"`
}

func (m *Manifest) HostName() string {
	if m.Host == nil {
		return ""
	}
	return m.Host.Name
}

func Parse(data []byte) (*Manifest, error) {
	var m Manifest
	if err := yaml.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	return &m, nil
}

func Load(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest %s: %w", path, err)
	}
	m, err := Parse(data)
	if err != nil {
		return nil, err
	}
	if err := m.Validate(); err != nil {
		return nil, fmt.Errorf("invalid manifest %s: %w", path, err)
	}
	return m, nil
}

func (m *Manifest) Validate() error {
	if m.Project == "" {
		return fmt.Errorf("manifest: project is required")
	}
	if m.DefaultBaseline == "" {
		return fmt.Errorf("manifest: defaultBaseline is required")
	}
	if len(m.Repos) == 0 {
		return fmt.Errorf("manifest: at least one repo is required")
	}
	seen := map[string]bool{}
	for i, r := range m.Repos {
		if r.Name == "" {
			return fmt.Errorf("manifest: repos[%d]: name is required", i)
		}
		if seen[r.Name] {
			return fmt.Errorf("manifest: duplicate repo name %q", r.Name)
		}
		seen[r.Name] = true
		if r.Remote == "" {
			return fmt.Errorf("manifest: repo %q: remote is required", r.Name)
		}
	}
	if m.Host != nil {
		if m.Host.Name == "" {
			return fmt.Errorf("manifest: host: name is required")
		}
		if m.Host.Remote == "" {
			return fmt.Errorf("manifest: host %q: remote is required", m.Host.Name)
		}
		if seen[m.Host.Name] {
			return fmt.Errorf("manifest: host name %q collides with a repo", m.Host.Name)
		}
	}
	return nil
}

func (m *Manifest) LabelFor(name string) string {
	if m.Host != nil && m.Host.Name == name {
		if m.Host.Label != "" {
			return m.Host.Label
		}
		return name
	}
	for _, r := range m.Repos {
		if r.Name == name {
			if r.Label != "" {
				return r.Label
			}
			return name
		}
	}
	return name
}

func (m *Manifest) BaselineFor(repoName string) (string, error) {
	if m.Host != nil && m.Host.Name == repoName {
		if m.Host.Baseline != "" {
			return m.Host.Baseline, nil
		}
		return m.DefaultBaseline, nil
	}
	for _, r := range m.Repos {
		if r.Name == repoName {
			if r.Baseline != "" {
				return r.Baseline, nil
			}
			return m.DefaultBaseline, nil
		}
	}
	return "", fmt.Errorf("manifest: unknown repo %q", repoName)
}
