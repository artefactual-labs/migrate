# List filter helper

A tiny utility that removes UUIDs listed in `to_filter_out.txt` from a source
file called `original_list.txt`, writing the result to `final_list.txt`.

## Usage

1. Place `original_list.txt` and `to_filter_out.txt` alongside the binary.
2. Run `go run ./cmd/list-filter` (or build the command first).
3. Use the generated `final_list.txt` as the cleaned list of UUIDs.

The program validates every UUID it reads and prints a quick tally of the
original, filtered, and final counts.
