package securitymaster

import (
	"fmt"
	"regexp"
)

var mysqlIdentifierPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)

func BootstrapDDL(database string) ([]string, error) {
	db, err := quoteMySQLIdent(database)
	if err != nil {
		return nil, err
	}
	return []string{
		fmt.Sprintf("CREATE DATABASE IF NOT EXISTS %s DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci", db),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.securities
(
    market varchar(8) NOT NULL,
    symbol varchar(16) NOT NULL,
    exchange varchar(16) NOT NULL,
    current_name varchar(128) NOT NULL,
    current_name_norm varchar(128) NOT NULL,
    board varchar(32) NOT NULL,
    status varchar(32) NOT NULL,
    listing_date date NULL,
    delisting_date date NULL,
    lot_size int NULL,
    price_precision tinyint NULL,
    source varchar(64) NOT NULL,
    manual_locked boolean NOT NULL DEFAULT false,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (market, symbol),
    KEY idx_securities_name_norm (current_name_norm),
    KEY idx_securities_status (status),
    KEY idx_securities_exchange_board (exchange, board)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`, db),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.security_name_history
(
    id bigint unsigned NOT NULL AUTO_INCREMENT,
    market varchar(8) NOT NULL,
    symbol varchar(16) NOT NULL,
    name varchar(128) NOT NULL,
    name_norm varchar(128) NOT NULL,
    valid_from date NOT NULL,
    valid_to date NULL,
    source varchar(64) NOT NULL,
    manual_override boolean NOT NULL DEFAULT false,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_name_history_segment (market, symbol, valid_from, name_norm),
    KEY idx_name_history_name_norm (name_norm),
    KEY idx_name_history_security_valid_from (market, symbol, valid_from)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`, db),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.security_aliases
(
    id bigint unsigned NOT NULL AUTO_INCREMENT,
    market varchar(8) NOT NULL,
    symbol varchar(16) NOT NULL,
    alias varchar(128) NOT NULL,
    alias_norm varchar(128) NOT NULL,
    alias_type varchar(32) NOT NULL,
    priority int NOT NULL DEFAULT 50,
    source varchar(64) NOT NULL,
    created_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    updated_at datetime(3) NOT NULL DEFAULT CURRENT_TIMESTAMP(3) ON UPDATE CURRENT_TIMESTAMP(3),
    PRIMARY KEY (id),
    UNIQUE KEY uk_security_alias (market, symbol, alias_norm, alias_type),
    KEY idx_security_alias_norm_priority (alias_norm, priority),
    KEY idx_security_alias_security (market, symbol)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`, db),
		fmt.Sprintf(`CREATE TABLE IF NOT EXISTS %s.security_refresh_runs
(
    id bigint unsigned NOT NULL AUTO_INCREMENT,
    source varchar(64) NOT NULL,
    markets varchar(64) NOT NULL,
    started_at datetime(3) NOT NULL,
    finished_at datetime(3) NULL,
    status varchar(32) NOT NULL,
    rows_seen int NOT NULL DEFAULT 0,
    rows_upserted int NOT NULL DEFAULT 0,
    rows_skipped int NOT NULL DEFAULT 0,
    aliases_upserted int NOT NULL DEFAULT 0,
    history_upserted int NOT NULL DEFAULT 0,
    error text NULL,
    PRIMARY KEY (id),
    KEY idx_refresh_started_at (started_at),
    KEY idx_refresh_source_status (source, status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci`, db),
	}, nil
}

func quoteMySQLIdent(value string) (string, error) {
	if !mysqlIdentifierPattern.MatchString(value) {
		return "", fmt.Errorf("invalid MySQL identifier %q", value)
	}
	return "`" + value + "`", nil
}
