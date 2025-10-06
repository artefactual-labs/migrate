package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffhelp"

	"github.com/artefactual-labs/migrate/internal/cmd/exportcmd"
	"github.com/artefactual-labs/migrate/internal/cmd/listfiltercmd"
	"github.com/artefactual-labs/migrate/internal/cmd/loadinputcmd"
	"github.com/artefactual-labs/migrate/internal/cmd/movecmd"
	"github.com/artefactual-labs/migrate/internal/cmd/replicatecmd"
	"github.com/artefactual-labs/migrate/internal/cmd/rootcmd"
	"github.com/artefactual-labs/migrate/internal/cmd/versioncmd"
	"github.com/artefactual-labs/migrate/internal/cmd/workercmd"
)

func main() {
	ctx := context.Background()
	args := os.Args[1:]
	stdin := os.Stdin
	stdout := os.Stdout
	stderr := os.Stderr

	if err := exec(ctx, args, stdin, stdout, stderr); err != nil {
		if errors.Is(err, ff.ErrHelp) {
			return
		}
		_, _ = fmt.Fprintf(stderr, "error: %v\n", err) //nolint:errcheck
		os.Exit(1)
	}
}

func exec(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	root := rootcmd.New(stdin, stdout, stderr)
	_ = exportcmd.New(root)
	_ = listfiltercmd.New(root)
	_ = loadinputcmd.New(root)
	_ = movecmd.New(root)
	_ = replicatecmd.New(root)
	_ = versioncmd.New(root)
	_ = workercmd.New(root)

	if err := root.Command.Parse(args); err != nil {
		_, _ = fmt.Fprintf(stderr, "\n%s\n", ffhelp.Command(root.Command))
		return err
	}

	return root.Command.Run(ctx)
}
