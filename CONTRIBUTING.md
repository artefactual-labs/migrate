# Contributing

## Prerequisites

Install [Go], [uv] and `make`.

## Pre-commit hooks

Run the hooks against all files before submitting changes:

```bash
uvx pre-commit run --all-files
```

Install the Git hook so checks run automatically:

```bash
uvx pre-commit install
```

[Go]: https://go.dev/dl/
[uv]: https://docs.astral.sh/uv/getting-started/installation/
