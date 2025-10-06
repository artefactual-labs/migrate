package application

import (
	"bufio"
	"errors"
	"fmt"
	"os"

	"github.com/google/uuid"
)

const InputFile = "input.txt"

func LoadInputUUIDs() ([]uuid.UUID, error) {
	return LoadUUIDsFromFile(InputFile)
}

func LoadUUIDsFromFile(path string) ([]uuid.UUID, error) {
	lines, err := readNonEmptyLines(path)
	if err != nil {
		return nil, err
	}

	uuids, err := ValidateUUIDs(lines)
	if err != nil {
		return nil, fmt.Errorf("validate UUIDs: %w", err)
	}

	return uuids, nil
}

func readNonEmptyLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, f.Close()) }()

	var lines []string
	scanner := bufio.NewScanner(f)
	scanner.Split(bufio.ScanLines)
	for scanner.Scan() {
		text := scanner.Text()
		if text == "" {
			continue
		}
		lines = append(lines, text)
	}

	return lines, errors.Join(err, scanner.Err())
}

func ReadNonEmptyLines(path string) ([]string, error) {
	return readNonEmptyLines(path)
}

func WriteLines(path string, lines []string) (err error) {
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
