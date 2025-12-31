package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const (
	// AuthFileName is the name of the authentication data file
	AuthFileName = "auth.json"
	// TinyscaleDir is the directory name for tinyscale data
	TinyscaleDir = ".tinyscale"
)

// GetAuthFilePath returns the path to the auth.json file
func GetAuthFilePath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("unable to get user home directory: %w", err)
	}
	return filepath.Join(homeDir, TinyscaleDir, AuthFileName), nil
}

// LoadAuthData loads the authentication data from ~/.tinyscale/auth.json
func LoadAuthData() (*AuthData, error) {
	authPath, err := GetAuthFilePath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(authPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No auth data exists yet
		}
		return nil, fmt.Errorf("unable to read auth file: %w", err)
	}

	var authData AuthData
	if err := json.Unmarshal(data, &authData); err != nil {
		return nil, fmt.Errorf("unable to parse auth file: %w", err)
	}

	return &authData, nil
}

// SaveAuthData saves the authentication data to ~/.tinyscale/auth.json
func SaveAuthData(authData *AuthData) error {
	authPath, err := GetAuthFilePath()
	if err != nil {
		return err
	}

	// Ensure the directory exists
	dir := filepath.Dir(authPath)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("unable to create auth directory: %w", err)
	}

	data, err := json.MarshalIndent(authData, "", "  ")
	if err != nil {
		return fmt.Errorf("unable to marshal auth data: %w", err)
	}

	if err := os.WriteFile(authPath, data, 0600); err != nil {
		return fmt.Errorf("unable to write auth file: %w", err)
	}

	return nil
}

// ClearAuthData removes the authentication data file
func ClearAuthData() error {
	authPath, err := GetAuthFilePath()
	if err != nil {
		return err
	}

	if err := os.Remove(authPath); err != nil {
		if os.IsNotExist(err) {
			return nil // File doesn't exist, nothing to clear
		}
		return fmt.Errorf("unable to remove auth file: %w", err)
	}

	return nil
}
