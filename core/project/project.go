package project

import (
	"fmt"
	"sort"

	"github.com/tttpeng/grove/core/config"
)

type Entry struct {
	Name     string
	Manifest string
	Current  bool
}

func List(cfg *config.Config) []Entry {
	names := make([]string, 0, len(cfg.Projects))
	for name := range cfg.Projects {
		names = append(names, name)
	}
	sort.Strings(names)
	entries := make([]Entry, 0, len(names))
	for _, name := range names {
		entries = append(entries, Entry{
			Name:     name,
			Manifest: cfg.Projects[name].Manifest,
			Current:  name == cfg.Current,
		})
	}
	return entries
}

func Use(cfg *config.Config, name string) error {
	if _, ok := cfg.Projects[name]; !ok {
		return fmt.Errorf("unknown project %q", name)
	}
	cfg.Current = name
	return nil
}

func Remove(cfg *config.Config, name string) error {
	if _, ok := cfg.Projects[name]; !ok {
		return fmt.Errorf("unknown project %q", name)
	}
	delete(cfg.Projects, name)
	if cfg.Current == name {
		cfg.Current = ""
	}
	return nil
}
