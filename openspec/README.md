# OpenSpec

`infinity-marketd` uses OpenSpec to describe capabilities before implementation.

## Layout

```text
openspec/
  specs/                  # accepted capability specs
  changes/                # proposed or in-progress changes
    <change-name>/
      proposal.md
      design.md
      tasks.md
      specs/
        <capability>/
          spec.md
```

## Rules

- Keep OpenSpec focused on testable capabilities and implementation changes.
- Do not copy general docs into OpenSpec.
- Prefer the simplest change that satisfies the current requirements.
- Derive specs from actual data formats, query patterns, and operator workflows.
- Use questions and validation tasks when a requirement is uncertain.
