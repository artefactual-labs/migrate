package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/artefactual-labs/migrate/internal/application"
)

func main() {
	var (
		args   = os.Args[1:]
		stdin  = os.Stdin
		stdout = os.Stdout
		stderr = os.Stderr
	)

	if err := exec(args, stdin, stdout, stderr); err != nil {
		printf(stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func exec(_ []string, _ io.Reader, stdout, _ io.Writer) error {
	filterList, err := readLines("to_filter_out.txt")
	if err != nil {
		return fmt.Errorf("read to_filter_out.txt: %v", err)
	}

	if _, err := application.ValidateUUIDs(filterList); err != nil {
		return fmt.Errorf("validate to_filter_out.txt: %v", err)
	}

	originalList, err := readLines("original_list.txt")
	if err != nil {
		return fmt.Errorf("read original_list.txt: %v", err)
	}
	if _, err := application.ValidateUUIDs(originalList); err != nil {
		return fmt.Errorf("validate original_list.txt: %v", err)
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

	if err := writeLines("final_list.txt", finalList); err != nil {
		return fmt.Errorf("write final_list.txt: %v", err)
	}

	printf(stdout, "Original Count: %d\n", len(originalList))
	printf(stdout, "To Filter Count: %d\n", len(filterList))
	printf(stdout, "Final Count: %d\n", len(finalList))

	return nil
}

// printf intentionally ignores write errors.
func printf(w io.Writer, format string, args ...any) {
	_, _ = fmt.Fprintf(w, format, args...)
}

// readLines reads a whole file into memory and returns a slice of its lines.
func readLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, f.Close()) }()

	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}

	return lines, errors.Join(err, scanner.Err())
}

// writeLines writes the lines to the given file.
func writeLines(path string, lines []string) (err error) {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, f.Close()) }()

	w := bufio.NewWriter(f)
	for _, line := range lines {
		if _, werr := fmt.Fprintln(w, line); werr != nil {
			return werr
		}
	}

	return errors.Join(err, w.Flush())
}
