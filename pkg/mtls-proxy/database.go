package mtlsproxy

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// DatabaseProvider handles database operations for authorization and routing
type DatabaseProvider struct {
	pool *pgxpool.Pool
}

// NewDatabaseProvider creates a new database provider
func NewDatabaseProvider(config *DatabaseConfig) (*DatabaseProvider, error) {
	poolConfig, err := pgxpool.ParseConfig(config.ConnectionString())
	if err != nil {
		return nil, fmt.Errorf("failed to parse database config: %w", err)
	}

	// Apply additional configuration
	poolConfig.MaxConns = int32(config.MaxOpenConns)
	poolConfig.MinConns = int32(config.MaxIdleConns / 2) // Reasonable default
	poolConfig.MaxConnLifetime = config.ConnMaxLifetime
	poolConfig.MaxConnIdleTime = config.ConnMaxIdleTime

	pool, err := pgxpool.NewWithConfig(context.Background(), poolConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Test the connection
	if err := pool.Ping(context.Background()); err != nil {
		pool.Close()
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return &DatabaseProvider{
		pool: pool,
	}, nil
}

// Close closes the database connection pool
func (d *DatabaseProvider) Close() {
	d.pool.Close()
}

// BackendHost represents a backend host from the database
type BackendHost struct {
	ConnectID      string
	InternalIPAddr string
	OrgID          string
	UserIDs        []string
	TeamIDs        []string
}

// GetBackendHostByConnectID retrieves backend host information by connect_id
func (d *DatabaseProvider) GetBackendHostByConnectID(ctx context.Context, connectID string) (*BackendHost, error) {
	statement := `SELECT connect_id, internal_ip_addr, org_id, user_ids, team_ids 
	              FROM backend_hosts 
	              WHERE connect_id = $1`

	row := d.pool.QueryRow(ctx, statement, connectID)

	var host BackendHost
	err := row.Scan(
		&host.ConnectID,
		&host.InternalIPAddr,
		&host.OrgID,
		&host.UserIDs,
		&host.TeamIDs,
	)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("no backend host found for connect_id '%s'", connectID)
	}

	if err != nil {
		return nil, fmt.Errorf("failed to query backend host: %w", err)
	}

	return &host, nil
}

// IsUserAuthorized checks if a user is authorized to access a backend host
// Authorization logic mirrors ssh-router:
// 1. User must be in the same organization as the host
// 2. User must be explicitly authorized either:
//   - Directly via user_ids list, OR
//   - Indirectly via team membership (team_ids)
func (d *DatabaseProvider) IsUserAuthorized(ctx context.Context, userID, orgID, connectID string) (bool, error) {
	// First, get the backend host information
	host, err := d.GetBackendHostByConnectID(ctx, connectID)
	if err != nil {
		return false, err
	}

	// Check organization match
	if host.OrgID != orgID {
		return false, nil
	}

	// Check if user is directly authorized
	for _, authorizedUserID := range host.UserIDs {
		if authorizedUserID == userID {
			return true, nil
		}
	}

	// Check if user is authorized via team membership
	if len(host.TeamIDs) > 0 {
		isTeamMember, err := d.isUserInTeams(ctx, userID, orgID, host.TeamIDs)
		if err != nil {
			return false, fmt.Errorf("failed to check team membership: %w", err)
		}
		if isTeamMember {
			return true, nil
		}
	}

	return false, nil
}

// isUserInTeams checks if a user is a member of any of the specified teams
func (d *DatabaseProvider) isUserInTeams(ctx context.Context, userID, orgID string, teamIDs []string) (bool, error) {
	if len(teamIDs) == 0 {
		return false, nil
	}

	// Build the query with placeholders for team IDs
	placeholders := make([]string, len(teamIDs))
	args := []any{orgID, userID}
	for i, teamID := range teamIDs {
		placeholders[i] = fmt.Sprintf("$%d", i+3)
		args = append(args, teamID)
	}

	statement := fmt.Sprintf(`
		SELECT EXISTS(
			SELECT 1 
			FROM teams 
			WHERE org_id = $1 
			  AND team_id IN (%s)
			  AND $2 = ANY(user_ids)
		)
	`, strings.Join(placeholders, ","))

	var exists bool
	err := d.pool.QueryRow(ctx, statement, args...).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check team membership: %w", err)
	}

	return exists, nil
}

// RouteTarget represents a routing target
type RouteTarget struct {
	BackendAddr string
	ConnectID   string
}

// RouteConnection determines the backend server address for a connection
func (d *DatabaseProvider) RouteConnection(ctx context.Context, userID, orgID, connectID string) (*RouteTarget, error) {
	// Check authorization first
	authorized, err := d.IsUserAuthorized(ctx, userID, orgID, connectID)
	if err != nil {
		return nil, fmt.Errorf("authorization check failed: %w", err)
	}

	if !authorized {
		return nil, fmt.Errorf("user '%s' is not authorized to access host '%s'", userID, connectID)
	}

	// Get the backend host
	host, err := d.GetBackendHostByConnectID(ctx, connectID)
	if err != nil {
		return nil, err
	}

	if host.InternalIPAddr == "" {
		return nil, fmt.Errorf("backend host '%s' has no internal IP address", connectID)
	}

	return &RouteTarget{
		BackendAddr: host.InternalIPAddr,
		ConnectID:   connectID,
	}, nil
}
