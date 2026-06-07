## ADDED Requirements

### Requirement: Remote financial manifest listing
The system SHALL list remote TDX professional financial report packages from a `gpcw.txt` manifest.

#### Scenario: List available financial packages
- **WHEN** an operator runs `marketd tdx-fin-files`
- **THEN** marketd MUST fetch `gpcw.txt` from the configured financial remote base URL
- **AND** it MUST parse each non-empty manifest line as `filename,md5,size`
- **AND** it MUST print filename, md5, and size for each package

#### Scenario: Reject malformed manifest lines
- **WHEN** the remote `gpcw.txt` manifest contains a non-empty line that cannot be parsed as `filename,md5,size`
- **THEN** marketd MUST fail the command with a clear manifest parse error

#### Scenario: Manifest filename safety
- **WHEN** a manifest line references an absolute path, parent directory path, or non-`gpcwYYYYMMDD.zip` filename
- **THEN** marketd MUST reject that line before any download is attempted

### Requirement: Remote financial package fetching
The system SHALL fetch remote TDX professional financial report packages into a local directory.

#### Scenario: Fetch selected package
- **WHEN** an operator runs `marketd tdx-fin-fetch --filename gpcw20251231.zip --dir data/tdxfin`
- **THEN** marketd MUST download `gpcw20251231.zip` from the configured financial remote base URL
- **AND** it MUST create the target directory when it does not exist
- **AND** it MUST write the package to `data/tdxfin/gpcw20251231.zip`

#### Scenario: Verify fetched package
- **WHEN** manifest metadata is available for a fetched package
- **THEN** marketd MUST verify the downloaded file size and MD5 against the manifest entry
- **AND** it MUST fail the command if either verification does not match

#### Scenario: Skip matching existing package
- **WHEN** the target file already exists and its size and MD5 match the manifest entry
- **THEN** marketd MUST skip the download and report the file as already present

#### Scenario: Explicit full manifest fetch
- **WHEN** an operator runs `marketd tdx-fin-fetch --all --dir data/tdxfin`
- **THEN** marketd MUST fetch every package listed in the manifest
- **AND** it MUST reject a command that omits both `--filename` and `--all`

### Requirement: Remote financial package parse validation
The system SHALL parse downloaded remote financial report packages without writing ClickHouse facts.

#### Scenario: Parse fetched package
- **WHEN** an operator runs `marketd tdx-fin-parse --file data/tdxfin/gpcw20251231.zip`
- **THEN** marketd MUST parse the package using the existing `gpcw` financial parser
- **AND** it MUST print a dry-run summary including files processed, dictionary count, rows, manifest issues, and quality issues
- **AND** it MUST NOT open ClickHouse or write financial facts

#### Scenario: Parse missing package
- **WHEN** an operator runs `marketd tdx-fin-parse --file` with a missing path
- **THEN** marketd MUST fail with a local file error

### Requirement: Remote financial workflow testability
The system SHALL keep remote financial workflow tests deterministic.

#### Scenario: Override remote base URL
- **WHEN** tests or operators pass `--base-url`
- **THEN** marketd MUST fetch `gpcw.txt` and package files from that base URL
- **AND** unit tests MUST use local fixture servers instead of live TDX endpoints
