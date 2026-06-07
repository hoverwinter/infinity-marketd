## ADDED Requirements

### Requirement: Decode GBK security names
The system SHALL decode TDX standard行情 security-list names from GB18030-compatible bytes.

#### Scenario: Chinese name
- **WHEN** a security-list record contains GBK bytes for a Chinese security name
- **THEN** the decoded `Security.Name` contains the correct Unicode string

#### Scenario: ASCII name
- **WHEN** a security-list record contains ASCII name bytes
- **THEN** the decoded `Security.Name` preserves the ASCII name

### Requirement: Trim fixed-field padding
The system SHALL trim fixed-field padding from decoded TDX security-list names.

#### Scenario: Null padding
- **WHEN** a security-list name field has trailing null bytes
- **THEN** the decoded `Security.Name` does not contain null characters

#### Scenario: Space padding
- **WHEN** a security-list name field has trailing spaces
- **THEN** the decoded `Security.Name` does not contain trailing spaces

### Requirement: Tolerate malformed names
The system SHALL keep security-list symbol discovery usable when a name field cannot be decoded.

#### Scenario: Invalid name bytes
- **WHEN** a security-list record has malformed name bytes but a valid six-digit symbol
- **THEN** the decoder returns the security record with an empty `Name`
- **AND** it does not fail the entire security-list response

### Requirement: Document security-list name encoding
The system SHALL document the TDX security-list name encoding behavior.

#### Scenario: Documentation updated
- **WHEN** GBK/GB18030 name decoding is implemented
- **THEN** realtime quote and TDX server reference docs describe the decoded security-list name behavior
