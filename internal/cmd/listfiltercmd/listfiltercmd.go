package listfiltercmd

import (
	"context"
	"fmt"
	"io"

	"github.com/peterbourgon/ff/v4"

	"github.com/artefactual-labs/migrate/internal/application"
	"github.com/artefactual-labs/migrate/internal/cmd/rootcmd"
)

type Config struct {
	*rootcmd.RootConfig
	Command *ff.Command
	Flags   *ff.FlagSet
}

func New(parent *rootcmd.RootConfig) *Config {
	cfg := &Config{RootConfig: parent}
	cfg.Flags = ff.NewFlagSet("list-filter").SetParent(parent.Flags)

	cfg.Command = &ff.Command{
		Name:      "list-filter",
		Usage:     "migrate list-filter",
		ShortHelp: "Filter UUIDs from original_list.txt based on to_filter_out.txt.",
		Flags:     cfg.Flags,
		Exec:      cfg.Exec,
	}

	parent.Command.Subcommands = append(parent.Command.Subcommands, cfg.Command)
	return cfg
}

func (cfg *Config) Exec(ctx context.Context, _ []string) error {
	filterList, err := application.ReadNonEmptyLines("to_filter_out.txt")
	if err != nil {
		return fmt.Errorf("read to_filter_out.txt: %w", err)
	}

	if _, err := application.ValidateUUIDs(filterList); err != nil {
		return fmt.Errorf("validate to_filter_out.txt: %w", err)
	}

	originalList, err := application.ReadNonEmptyLines("original_list.txt")
	if err != nil {
		return fmt.Errorf("read original_list.txt: %w", err)
	}

	if _, err := application.ValidateUUIDs(originalList); err != nil {
		return fmt.Errorf("validate original_list.txt: %w", err)
	}

	filterSet := make(map[string]struct{}, len(filterList))
	for _, v := range filterList {
		filterSet[v] = struct{}{}
	}

	finalList := make([]string, 0, len(originalList))
	for _, v := range originalList {
		if _, exists := filterSet[v]; exists {
			continue
		}
		finalList = append(finalList, v)
	}

	if err := application.WriteLines("final_list.txt", finalList); err != nil {
		return fmt.Errorf("write final_list.txt: %w", err)
	}

	printf(cfg.Stdout, "Original Count: %d\n", len(originalList))
	printf(cfg.Stdout, "To Filter Count: %d\n", len(filterList))
	printf(cfg.Stdout, "Final Count: %d\n", len(finalList))

	return nil
}

func printf(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...)
}
