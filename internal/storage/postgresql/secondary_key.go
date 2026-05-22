package postgresql

import (
	"context"
	"database/sql"
	"errors"
)

func (s *Store) InitializeSecondaryKeyTable() error {
	createTableQuery := `
	CREATE TABLE IF NOT EXISTS secondary_keys (
	    secondary_hash VARCHAR(255) PRIMARY KEY,
	    key_hash VARCHAR(255)
	)`

	ctxTimeout, cancel := context.WithTimeout(context.Background(), s.wt)
	defer cancel()
	_, err := s.db.ExecContext(ctxTimeout, createTableQuery)
	if err != nil {
		return err
	}
	return nil
}

func (s *Store) GetKeyHashBySecondary(sHash string) (string, error) {
	ctxTimeout, cancel := context.WithTimeout(context.Background(), s.rt)
	defer cancel()
	query := "SELECT key_hash FROM secondary_keys WHERE secondary_hash = $1"

	row := s.db.QueryRowContext(ctxTimeout, query, sHash)

	var keyHash sql.NullString
	err := row.Scan(&keyHash)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return "", sql.ErrNoRows
		}
		return "", err
	}

	if !keyHash.Valid {
		return "", sql.ErrNoRows
	}

	return keyHash.String, nil
}

func (s *Store) CreateSecondaryKey(secondaryHash string) error {
	ctxTimeout, cancel := context.WithTimeout(context.Background(), s.wt)
	defer cancel()

	query := "INSERT INTO secondary_keys (secondary_hash) VALUES ($1)"

	_, err := s.db.ExecContext(ctxTimeout, query, secondaryHash)
	return err
}

func (s *Store) UpdateSecondaryKey(secondaryHash, keyHash string) error {
	ctxTimeout, cancel := context.WithTimeout(context.Background(), s.wt)
	defer cancel()

	query := "UPDATE secondary_keys SET key_hash = $1 WHERE secondary_hash = $2"

	result, err := s.db.ExecContext(ctxTimeout, query, keyHash, secondaryHash)
	if err != nil {
		return err
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil

}
