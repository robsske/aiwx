// Package repository provides database access for AIWX.
package repository

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/rverhe/aiwx/internal/auth"
	"github.com/rverhe/aiwx/internal/domain"
)

// Pool wraps pgxpool for type-safe database access.
type Pool struct {
	*pgxpool.Pool
}

// NewPool creates a new connection pool.
func NewPool(databaseURL string) (*Pool, error) {
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database url: %w", err)
	}
	cfg.MaxConns = 20
	cfg.MinConns = 5
	cfg.MaxConnLifetime = time.Hour

	pool, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		return nil, fmt.Errorf("create pool: %w", err)
	}
	if err := pool.Ping(context.Background()); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}
	return &Pool{pool}, nil
}

// ─── Health ───────────────────────────────────────────────────────────────────

// Health checks database connectivity.
func (p *Pool) Health(ctx context.Context) error {
	return p.Ping(ctx)
}

// ─── Organizations ────────────────────────────────────────────────────────────

type OrganizationRepo struct {
	pool *Pool
}

func NewOrganizationRepo(pool *Pool) *OrganizationRepo {
	return &OrganizationRepo{pool: pool}
}

func (r *OrganizationRepo) Create(ctx context.Context, input domain.OrganizationCreate) (*domain.Organization, error) {
	org := &domain.Organization{
		ID:          uuid.New(),
		Name:        input.Name,
		DisplayName: input.DisplayName,
		Status:      "active",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	_, err := r.pool.Exec(ctx,
		`INSERT INTO organizations (id, name, display_name, status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		org.ID, org.Name, org.DisplayName, org.Status, org.CreatedAt, org.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert organization: %w", err)
	}
	return org, nil
}

func (r *OrganizationRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Organization, error) {
	org := &domain.Organization{}
	err := r.pool.QueryRow(ctx,
		`SELECT id, name, display_name, status, created_at, updated_at
		 FROM organizations WHERE id = $1`, id,
	).Scan(&org.ID, &org.Name, &org.DisplayName, &org.Status, &org.CreatedAt, &org.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return org, nil
}

func (r *OrganizationRepo) List(ctx context.Context, limit int, cursor string) ([]domain.Organization, string, bool, error) {
	// Simple implementation: list all, paginate by cursor
	rows, err := r.pool.Query(ctx,
		`SELECT id, name, display_name, status, created_at, updated_at
		 FROM organizations
		 WHERE ($1 = '' OR id > $1::uuid)
		 ORDER BY id
		 LIMIT $2`, cursor, limit+1,
	)
	if err != nil {
		return nil, "", false, err
	}
	defer rows.Close()

	var orgs []domain.Organization
	for rows.Next() {
		var org domain.Organization
		if err := rows.Scan(&org.ID, &org.Name, &org.DisplayName, &org.Status, &org.CreatedAt, &org.UpdatedAt); err != nil {
			return nil, "", false, err
		}
		orgs = append(orgs, org)
	}

	hasMore := len(orgs) > limit
	if hasMore {
		orgs = orgs[:limit]
	}
	nextCursor := ""
	if len(orgs) > 0 {
		nextCursor = orgs[len(orgs)-1].ID.String()
	}
	return orgs, nextCursor, hasMore, nil
}

// ListByIDs returns only the organizations matching the given IDs.
func (r *OrganizationRepo) ListByIDs(ctx context.Context, ids []uuid.UUID, limit int, cursor string) ([]domain.Organization, string, bool, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, name, display_name, status, created_at, updated_at
		 FROM organizations
		 WHERE id = ANY($1) AND ($2 = '' OR id > $2::uuid)
		 ORDER BY id
		 LIMIT $3`, ids, cursor, limit+1,
	)
	if err != nil {
		return nil, "", false, err
	}
	defer rows.Close()

	var orgs []domain.Organization
	for rows.Next() {
		var org domain.Organization
		if err := rows.Scan(&org.ID, &org.Name, &org.DisplayName, &org.Status, &org.CreatedAt, &org.UpdatedAt); err != nil {
			return nil, "", false, err
		}
		orgs = append(orgs, org)
	}

	hasMore := len(orgs) > limit
	if hasMore {
		orgs = orgs[:limit]
	}
	nextCursor := ""
	if len(orgs) > 0 {
		nextCursor = orgs[len(orgs)-1].ID.String()
	}
	return orgs, nextCursor, hasMore, nil
}

// ─── Users ────────────────────────────────────────────────────────────────────

type UserRepo struct {
	pool *Pool
}

func NewUserRepo(pool *Pool) *UserRepo {
	return &UserRepo{pool: pool}
}

func (r *UserRepo) Create(ctx context.Context, input domain.UserCreate) (*domain.User, error) {
	user := &domain.User{
		ID:          uuid.New(),
		Email:       input.Email,
		DisplayName: input.DisplayName,
		IsActive:    true,
		CreatedAt:   time.Now(),
	}
	var passwordHash *string
	if input.Password != "" {
		hash, err := auth.HashPassword(input.Password)
		if err != nil {
			return nil, fmt.Errorf("hash password: %w", err)
		}
		passwordHash = &hash
	}
	_, err := r.pool.Exec(ctx,
		`INSERT INTO users (id, email, display_name, password_hash, is_active, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		user.ID, user.Email, user.DisplayName, passwordHash, user.IsActive, user.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert user: %w", err)
	}
	return user, nil
}

// Credential is an auth-lookup result including the password hash.
type Credential struct {
	User         *domain.User
	PasswordHash string
}

// GetCredentialByEmail loads a user and their password hash for authentication.
func (r *UserRepo) GetCredentialByEmail(ctx context.Context, email string) (*Credential, error) {
	cred := &Credential{User: &domain.User{}}
	var passwordHash *string
	err := r.pool.QueryRow(ctx,
		`SELECT id, email, display_name, password_hash, is_active, created_at
		 FROM users WHERE email = $1`, email,
	).Scan(&cred.User.ID, &cred.User.Email, &cred.User.DisplayName, &passwordHash, &cred.User.IsActive, &cred.User.CreatedAt)
	if err != nil {
		return nil, err
	}
	if passwordHash != nil {
		cred.PasswordHash = *passwordHash
	}
	return cred, nil
}

// SetPassword updates a user's password hash.
func (r *UserRepo) SetPassword(ctx context.Context, userID uuid.UUID, passwordHash string) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE users SET password_hash = $2, updated_at = now() WHERE id = $1`,
		userID, passwordHash,
	)
	return err
}

func (r *UserRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	user := &domain.User{}
	err := r.pool.QueryRow(ctx,
		`SELECT id, email, display_name, is_active, created_at
		 FROM users WHERE id = $1`, id,
	).Scan(&user.ID, &user.Email, &user.DisplayName, &user.IsActive, &user.CreatedAt)
	if err != nil {
		return nil, err
	}
	return user, nil
}

func (r *UserRepo) GetByEmail(ctx context.Context, email string) (*domain.User, error) {
	user := &domain.User{}
	err := r.pool.QueryRow(ctx,
		`SELECT id, email, display_name, is_active, created_at
		 FROM users WHERE email = $1`, email,
	).Scan(&user.ID, &user.Email, &user.DisplayName, &user.IsActive, &user.CreatedAt)
	if err != nil {
		return nil, err
	}
	return user, nil
}

// ─── Projects ─────────────────────────────────────────────────────────────────

type ProjectRepo struct {
	pool *Pool
}

func NewProjectRepo(pool *Pool) *ProjectRepo {
	return &ProjectRepo{pool: pool}
}

func (r *ProjectRepo) Create(ctx context.Context, input domain.ProjectCreate, orgID uuid.UUID) (*domain.Project, error) {
	project := &domain.Project{
		ID:             uuid.New(),
		Name:           input.Name,
		OrganizationID: orgID,
		SCMURL:         input.SCMURL,
		SCMBranch:      input.SCMBranch,
		SCMCredentialID: input.SCMCredentialID,
		SyncStatus:     "pending",
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	if project.SCMBranch == "" {
		project.SCMBranch = "main"
	}
	_, err := r.pool.Exec(ctx,
		`INSERT INTO projects (id, name, organization_id, scm_url, scm_branch, scm_credential_id, sync_status, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`,
		project.ID, project.Name, project.OrganizationID, project.SCMURL,
		project.SCMBranch, project.SCMCredentialID, project.SyncStatus,
		project.CreatedAt, project.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert project: %w", err)
	}
	return project, nil
}

func (r *ProjectRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Project, error) {
	p := &domain.Project{}
	err := r.pool.QueryRow(ctx,
		`SELECT id, name, organization_id, scm_url, scm_branch, scm_credential_id,
		        latest_revision, sync_status, last_sync_at, created_at, updated_at
		 FROM projects WHERE id = $1`, id,
	).Scan(&p.ID, &p.Name, &p.OrganizationID, &p.SCMURL, &p.SCMBranch,
		&p.SCMCredentialID, &p.LatestRevision, &p.SyncStatus, &p.LastSyncAt,
		&p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return p, nil
}

// ─── Teams ────────────────────────────────────────────────────────────────────

type TeamRepo struct {
	pool *Pool
}

func NewTeamRepo(pool *Pool) *TeamRepo {
	return &TeamRepo{pool: pool}
}

func (r *TeamRepo) Create(ctx context.Context, input domain.TeamCreate, orgID uuid.UUID) (*domain.Team, error) {
	team := &domain.Team{
		ID:             uuid.New(),
		Name:           input.Name,
		DisplayName:    input.DisplayName,
		OrganizationID: orgID,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	_, err := r.pool.Exec(ctx,
		`INSERT INTO teams (id, organization_id, name, display_name, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		team.ID, team.OrganizationID, team.Name, team.DisplayName, team.CreatedAt, team.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert team: %w", err)
	}
	return team, nil
}

func (r *TeamRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Team, error) {
	team := &domain.Team{}
	err := r.pool.QueryRow(ctx,
		`SELECT t.id, t.name, t.display_name, t.organization_id, t.created_at, t.updated_at,
		        (SELECT COUNT(*) FROM team_members tm WHERE tm.team_id = t.id)
		 FROM teams t WHERE t.id = $1`, id,
	).Scan(&team.ID, &team.Name, &team.DisplayName, &team.OrganizationID,
		&team.CreatedAt, &team.UpdatedAt, &team.MemberCount)
	if err != nil {
		return nil, err
	}
	return team, nil
}

func (r *TeamRepo) GetByIDScoped(ctx context.Context, id uuid.UUID, scope domain.Scope) (*domain.Team, error) {
	if scope.IsPlatformAdmin {
		return r.GetByID(ctx, id)
	}
	team := &domain.Team{}
	err := r.pool.QueryRow(ctx,
		`SELECT t.id, t.name, t.display_name, t.organization_id, t.created_at, t.updated_at,
		        (SELECT COUNT(*) FROM team_members tm WHERE tm.team_id = t.id)
		 FROM teams t WHERE t.id = $1 AND t.organization_id = ANY($2)`, id, scope.OrgIDs,
	).Scan(&team.ID, &team.Name, &team.DisplayName, &team.OrganizationID,
		&team.CreatedAt, &team.UpdatedAt, &team.MemberCount)
	if err != nil {
		return nil, err
	}
	return team, nil
}

func (r *TeamRepo) ListByOrg(ctx context.Context, orgID uuid.UUID) ([]domain.Team, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT t.id, t.name, t.display_name, t.organization_id, t.created_at, t.updated_at,
		        (SELECT COUNT(*) FROM team_members tm WHERE tm.team_id = t.id)
		 FROM teams t WHERE t.organization_id = $1 ORDER BY t.name`, orgID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var teams []domain.Team
	for rows.Next() {
		var team domain.Team
		if err := rows.Scan(&team.ID, &team.Name, &team.DisplayName, &team.OrganizationID,
			&team.CreatedAt, &team.UpdatedAt, &team.MemberCount); err != nil {
			return nil, err
		}
		teams = append(teams, team)
	}
	return teams, nil
}

func (r *TeamRepo) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM teams WHERE id = $1`, id)
	return err
}

func (r *TeamRepo) DeleteScoped(ctx context.Context, id uuid.UUID, scope domain.Scope) error {
	if scope.IsPlatformAdmin {
		return r.Delete(ctx, id)
	}
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM teams WHERE id = $1 AND organization_id = ANY($2)`,
		id, scope.OrgIDs,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *TeamRepo) AddMember(ctx context.Context, teamID, userID uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO team_members (team_id, user_id) VALUES ($1, $2)
		 ON CONFLICT DO NOTHING`, teamID, userID)
	return err
}

func (r *TeamRepo) RemoveMember(ctx context.Context, teamID, userID uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM team_members WHERE team_id = $1 AND user_id = $2`, teamID, userID)
	return err
}

// ─── Role Assignments ─────────────────────────────────────────────────────────

type RoleAssignmentRepo struct {
	pool *Pool
}

func NewRoleAssignmentRepo(pool *Pool) *RoleAssignmentRepo {
	return &RoleAssignmentRepo{pool: pool}
}

func (r *RoleAssignmentRepo) Create(ctx context.Context, input domain.RoleAssignmentCreate, orgID uuid.UUID) (*domain.RoleAssignment, error) {
	// Look up role ID
	var roleID uuid.UUID
	err := r.pool.QueryRow(ctx,
		`SELECT id FROM roles WHERE name = $1 AND (organization_id = $2 OR organization_id IS NULL)`,
		input.RoleName, orgID,
	).Scan(&roleID)
	if err != nil {
		return nil, fmt.Errorf("role not found: %w", err)
	}

	assignment := &domain.RoleAssignment{
		ID:             uuid.New(),
		RoleName:       input.RoleName,
		UserID:         input.UserID,
		TeamID:         input.TeamID,
		OrganizationID: orgID,
		CreatedAt:      time.Now(),
	}

	_, err = r.pool.Exec(ctx,
		`INSERT INTO role_assignments (id, role_id, user_id, team_id, organization_id, created_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		assignment.ID, roleID, assignment.UserID, assignment.TeamID,
		assignment.OrganizationID, assignment.CreatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert role assignment: %w", err)
	}
	return assignment, nil
}

func (r *RoleAssignmentRepo) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM role_assignments WHERE id = $1`, id)
	return err
}

// ListRolesForUser returns all role assignments for a user across all orgs.
func (r *RoleAssignmentRepo) ListRolesForUser(ctx context.Context, userID uuid.UUID) ([]domain.EffectiveRole, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT r.name, ra.organization_id, ra.team_id
		 FROM role_assignments ra
		 JOIN roles r ON r.id = ra.role_id
		 WHERE ra.user_id = $1
		 ORDER BY ra.created_at`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roles []domain.EffectiveRole
	for rows.Next() {
		var er domain.EffectiveRole
		if err := rows.Scan(&er.Role, &er.OrgID, &er.TeamID); err != nil {
			return nil, err
		}
		roles = append(roles, er)
	}
	return roles, rows.Err()
}

// EffectiveRoles returns all roles for a user, expanding team-scoped roles
// to the user through team membership.
func (r *RoleAssignmentRepo) EffectiveRoles(ctx context.Context, userID uuid.UUID) ([]domain.EffectiveRole, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT r.name, ra.organization_id, ra.team_id
		 FROM role_assignments ra
		 JOIN roles r ON r.id = ra.role_id
		 WHERE ra.user_id = $1
		   OR (ra.team_id IS NOT NULL AND ra.team_id IN (
		       SELECT tm.team_id FROM team_members tm WHERE tm.user_id = $1
		   ))
		 ORDER BY ra.created_at`,
		userID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var roles []domain.EffectiveRole
	for rows.Next() {
		var er domain.EffectiveRole
		if err := rows.Scan(&er.Role, &er.OrgID, &er.TeamID); err != nil {
			return nil, err
		}
		roles = append(roles, er)
	}
	return roles, rows.Err()
}

// ─── Credentials ──────────────────────────────────────────────────────────────

type CredentialRepo struct {
	pool *Pool
}

func NewCredentialRepo(pool *Pool) *CredentialRepo {
	return &CredentialRepo{pool: pool}
}

func (r *CredentialRepo) Create(ctx context.Context, input domain.CredentialCreate, orgID uuid.UUID) (*domain.Credential, error) {
	cred := &domain.Credential{
		ID:               uuid.New(),
		Name:             input.Name,
		OrganizationID:   orgID,
		CredentialTypeID: input.CredentialTypeID,
		Inputs:           input.Inputs,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}
	_, err := r.pool.Exec(ctx,
		`INSERT INTO credentials (id, organization_id, name, credential_type_id, inputs, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		cred.ID, cred.OrganizationID, cred.Name, cred.CredentialTypeID,
		cred.Inputs, cred.CreatedAt, cred.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert credential: %w", err)
	}
	// Redact secret values before returning
	cred.Inputs = redactSecrets(cred.Inputs)
	return cred, nil
}

func (r *CredentialRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Credential, error) {
	cred := &domain.Credential{}
	err := r.pool.QueryRow(ctx,
		`SELECT id, organization_id, name, credential_type_id, inputs, created_at, updated_at
		 FROM credentials WHERE id = $1`, id,
	).Scan(&cred.ID, &cred.OrganizationID, &cred.Name, &cred.CredentialTypeID,
		&cred.Inputs, &cred.CreatedAt, &cred.UpdatedAt)
	if err != nil {
		return nil, err
	}
	cred.Inputs = redactSecrets(cred.Inputs)
	return cred, nil
}

func (r *CredentialRepo) GetByIDScoped(ctx context.Context, id uuid.UUID, scope domain.Scope) (*domain.Credential, error) {
	if scope.IsPlatformAdmin {
		return r.GetByID(ctx, id)
	}
	cred := &domain.Credential{}
	err := r.pool.QueryRow(ctx,
		`SELECT id, organization_id, name, credential_type_id, inputs, created_at, updated_at
		 FROM credentials WHERE id = $1 AND organization_id = ANY($2)`, id, scope.OrgIDs,
	).Scan(&cred.ID, &cred.OrganizationID, &cred.Name, &cred.CredentialTypeID,
		&cred.Inputs, &cred.CreatedAt, &cred.UpdatedAt)
	if err != nil {
		return nil, err
	}
	cred.Inputs = redactSecrets(cred.Inputs)
	return cred, nil
}

func (r *CredentialRepo) ListByOrg(ctx context.Context, orgID uuid.UUID) ([]domain.Credential, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, organization_id, name, credential_type_id, inputs, created_at, updated_at
		 FROM credentials WHERE organization_id = $1 ORDER BY name`, orgID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var creds []domain.Credential
	for rows.Next() {
		var cred domain.Credential
		if err := rows.Scan(&cred.ID, &cred.OrganizationID, &cred.Name, &cred.CredentialTypeID,
			&cred.Inputs, &cred.CreatedAt, &cred.UpdatedAt); err != nil {
			return nil, err
		}
		cred.Inputs = redactSecrets(cred.Inputs)
		creds = append(creds, cred)
	}
	return creds, nil
}

func (r *CredentialRepo) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM credentials WHERE id = $1`, id)
	return err
}

func (r *CredentialRepo) DeleteScoped(ctx context.Context, id uuid.UUID, scope domain.Scope) error {
	if scope.IsPlatformAdmin {
		return r.Delete(ctx, id)
	}
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM credentials WHERE id = $1 AND organization_id = ANY($2)`,
		id, scope.OrgIDs,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// ListTypes returns available credential type definitions.
func (r *CredentialRepo) ListTypes(ctx context.Context) ([]domain.CredentialType, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, name, namespace, inputs_schema FROM credential_types ORDER BY name`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var types []domain.CredentialType
	for rows.Next() {
		var t domain.CredentialType
		if err := rows.Scan(&t.ID, &t.Name, &t.Namespace, &t.InputsSchema); err != nil {
			return nil, err
		}
		types = append(types, t)
	}
	return types, rows.Err()
}

// redactSecrets replaces secret values with "***" for API responses.
func redactSecrets(inputs map[string]any) map[string]any {
	if inputs == nil {
		return nil
	}
	redacted := make(map[string]any, len(inputs))
	for k, v := range inputs {
		if isSecretField(k) {
			redacted[k] = "***"
		} else {
			redacted[k] = v
		}
	}
	return redacted
}

func isSecretField(name string) bool {
	secretFields := []string{"password", "token", "secret", "key", "ssh_key", "private_key", "access_key"}
	for _, sf := range secretFields {
		if len(name) >= len(sf) && name[len(name)-len(sf):] == sf {
			return true
		}
	}
	return false
}

// ─── Inventories ──────────────────────────────────────────────────────────────

type InventoryRepo struct {
	pool *Pool
}

func NewInventoryRepo(pool *Pool) *InventoryRepo {
	return &InventoryRepo{pool: pool}
}

func (r *InventoryRepo) Create(ctx context.Context, input domain.InventoryCreate, orgID uuid.UUID) (*domain.Inventory, error) {
	inv := &domain.Inventory{
		ID:             uuid.New(),
		Name:           input.Name,
		Description:    input.Description,
		OrganizationID: orgID,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}
	_, err := r.pool.Exec(ctx,
		`INSERT INTO inventories (id, organization_id, name, description, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		inv.ID, inv.OrganizationID, inv.Name, inv.Description, inv.CreatedAt, inv.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert inventory: %w", err)
	}
	return inv, nil
}

func (r *InventoryRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.Inventory, error) {
	inv := &domain.Inventory{}
	err := r.pool.QueryRow(ctx,
		`SELECT i.id, i.name, i.description, i.organization_id, i.has_active_inventory_sources,
		        i.created_at, i.updated_at,
		        (SELECT COUNT(*) FROM inventory_hosts h WHERE h.inventory_id = i.id),
		        (SELECT COUNT(*) FROM inventory_groups g WHERE g.inventory_id = i.id)
		 FROM inventories i WHERE i.id = $1`, id,
	).Scan(&inv.ID, &inv.Name, &inv.Description, &inv.OrganizationID,
		&inv.HasActiveInventorySources, &inv.CreatedAt, &inv.UpdatedAt,
		&inv.HostCount, &inv.GroupCount)
	if err != nil {
		return nil, err
	}
	return inv, nil
}

func (r *InventoryRepo) GetByIDScoped(ctx context.Context, id uuid.UUID, scope domain.Scope) (*domain.Inventory, error) {
	if scope.IsPlatformAdmin {
		return r.GetByID(ctx, id)
	}
	inv := &domain.Inventory{}
	err := r.pool.QueryRow(ctx,
		`SELECT i.id, i.name, i.description, i.organization_id, i.has_active_inventory_sources,
		        i.created_at, i.updated_at,
		        (SELECT COUNT(*) FROM inventory_hosts h WHERE h.inventory_id = i.id),
		        (SELECT COUNT(*) FROM inventory_groups g WHERE g.inventory_id = i.id)
		 FROM inventories i WHERE i.id = $1 AND i.organization_id = ANY($2)`, id, scope.OrgIDs,
	).Scan(&inv.ID, &inv.Name, &inv.Description, &inv.OrganizationID,
		&inv.HasActiveInventorySources, &inv.CreatedAt, &inv.UpdatedAt,
		&inv.HostCount, &inv.GroupCount)
	if err != nil {
		return nil, err
	}
	return inv, nil
}

func (r *InventoryRepo) ListByOrg(ctx context.Context, orgID uuid.UUID) ([]domain.Inventory, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT i.id, i.name, i.description, i.organization_id, i.has_active_inventory_sources,
		        i.created_at, i.updated_at,
		        (SELECT COUNT(*) FROM inventory_hosts h WHERE h.inventory_id = i.id),
		        (SELECT COUNT(*) FROM inventory_groups g WHERE g.inventory_id = i.id)
		 FROM inventories i WHERE i.organization_id = $1 ORDER BY i.name`, orgID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var invs []domain.Inventory
	for rows.Next() {
		var inv domain.Inventory
		if err := rows.Scan(&inv.ID, &inv.Name, &inv.Description, &inv.OrganizationID,
			&inv.HasActiveInventorySources, &inv.CreatedAt, &inv.UpdatedAt,
			&inv.HostCount, &inv.GroupCount); err != nil {
			return nil, err
		}
		invs = append(invs, inv)
	}
	return invs, nil
}

func (r *InventoryRepo) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM inventories WHERE id = $1`, id)
	return err
}

func (r *InventoryRepo) DeleteScoped(ctx context.Context, id uuid.UUID, scope domain.Scope) error {
	if scope.IsPlatformAdmin {
		return r.Delete(ctx, id)
	}
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM inventories WHERE id = $1 AND organization_id = ANY($2)`,
		id, scope.OrgIDs,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *InventoryRepo) ListHosts(ctx context.Context, inventoryID uuid.UUID) ([]domain.InventoryHost, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, name, variables, enabled FROM inventory_hosts
		 WHERE inventory_id = $1 ORDER BY name`, inventoryID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var hosts []domain.InventoryHost
	for rows.Next() {
		var h domain.InventoryHost
		if err := rows.Scan(&h.ID, &h.Name, &h.Variables, &h.Enabled); err != nil {
			return nil, err
		}
		hosts = append(hosts, h)
	}
	return hosts, nil
}

func (r *InventoryRepo) AddHost(ctx context.Context, inventoryID uuid.UUID, name string, variables map[string]any) (*domain.InventoryHost, error) {
	host := &domain.InventoryHost{
		ID:        uuid.New(),
		Name:      name,
		Variables: variables,
		Enabled:   true,
	}
	_, err := r.pool.Exec(ctx,
		`INSERT INTO inventory_hosts (id, inventory_id, name, variables, enabled)
		 VALUES ($1, $2, $3, $4, $5)`,
		host.ID, inventoryID, host.Name, host.Variables, host.Enabled,
	)
	if err != nil {
		return nil, fmt.Errorf("insert host: %w", err)
	}
	return host, nil
}

func (r *InventoryRepo) RemoveHost(ctx context.Context, inventoryID, hostID uuid.UUID) error {
	_, err := r.pool.Exec(ctx,
		`DELETE FROM inventory_hosts WHERE id = $1 AND inventory_id = $2`, hostID, inventoryID)
	return err
}

// ─── Execution Environments ───────────────────────────────────────────────────

type ExecutionEnvironmentRepo struct {
	pool *Pool
}

func NewExecutionEnvironmentRepo(pool *Pool) *ExecutionEnvironmentRepo {
	return &ExecutionEnvironmentRepo{pool: pool}
}

func (r *ExecutionEnvironmentRepo) Create(ctx context.Context, input domain.ExecutionEnvironmentCreate, orgID uuid.UUID) (*domain.ExecutionEnvironment, error) {
	ee := &domain.ExecutionEnvironment{
		ID:         uuid.New(),
		Name:       input.Name,
		Image:      input.Image,
		Digest:     input.Digest,
		PullPolicy: input.PullPolicy,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}
	if ee.PullPolicy == "" {
		ee.PullPolicy = "IfNotPresent"
	}
	if orgID != uuid.Nil {
		ee.OrganizationID = &orgID
	}
	_, err := r.pool.Exec(ctx,
		`INSERT INTO execution_environments (id, organization_id, name, image, digest, pull_policy, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		ee.ID, ee.OrganizationID, ee.Name, ee.Image, ee.Digest, ee.PullPolicy, ee.CreatedAt, ee.UpdatedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert execution environment: %w", err)
	}
	return ee, nil
}

func (r *ExecutionEnvironmentRepo) GetByID(ctx context.Context, id uuid.UUID) (*domain.ExecutionEnvironment, error) {
	ee := &domain.ExecutionEnvironment{}
	err := r.pool.QueryRow(ctx,
		`SELECT id, organization_id, name, image, digest, pull_policy, created_at, updated_at
		 FROM execution_environments WHERE id = $1`, id,
	).Scan(&ee.ID, &ee.OrganizationID, &ee.Name, &ee.Image, &ee.Digest,
		&ee.PullPolicy, &ee.CreatedAt, &ee.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return ee, nil
}

func (r *ExecutionEnvironmentRepo) GetByIDScoped(ctx context.Context, id uuid.UUID, scope domain.Scope) (*domain.ExecutionEnvironment, error) {
	if scope.IsPlatformAdmin {
		return r.GetByID(ctx, id)
	}
	ee := &domain.ExecutionEnvironment{}
	err := r.pool.QueryRow(ctx,
		`SELECT id, organization_id, name, image, digest, pull_policy, created_at, updated_at
		 FROM execution_environments WHERE id = $1
		   AND (organization_id = ANY($2) OR organization_id IS NULL)`,
		id, scope.OrgIDs,
	).Scan(&ee.ID, &ee.OrganizationID, &ee.Name, &ee.Image, &ee.Digest,
		&ee.PullPolicy, &ee.CreatedAt, &ee.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return ee, nil
}

func (r *ExecutionEnvironmentRepo) ListByOrg(ctx context.Context, orgID uuid.UUID) ([]domain.ExecutionEnvironment, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, organization_id, name, image, digest, pull_policy, created_at, updated_at
		 FROM execution_environments
		 WHERE organization_id = $1 OR organization_id IS NULL
		 ORDER BY name`, orgID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ees []domain.ExecutionEnvironment
	for rows.Next() {
		var ee domain.ExecutionEnvironment
		if err := rows.Scan(&ee.ID, &ee.OrganizationID, &ee.Name, &ee.Image, &ee.Digest,
			&ee.PullPolicy, &ee.CreatedAt, &ee.UpdatedAt); err != nil {
			return nil, err
		}
		ees = append(ees, ee)
	}
	return ees, nil
}

func (r *ExecutionEnvironmentRepo) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM execution_environments WHERE id = $1`, id)
	return err
}

func (r *ExecutionEnvironmentRepo) DeleteScoped(ctx context.Context, id uuid.UUID, scope domain.Scope) error {
	if scope.IsPlatformAdmin {
		return r.Delete(ctx, id)
	}
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM execution_environments WHERE id = $1
		   AND (organization_id = ANY($2) OR organization_id IS NULL)`,
		id, scope.OrgIDs,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

// ─── Project (extended) ───────────────────────────────────────────────────────

func (r *ProjectRepo) GetByIDScoped(ctx context.Context, id uuid.UUID, scope domain.Scope) (*domain.Project, error) {
	if scope.IsPlatformAdmin {
		return r.GetByID(ctx, id)
	}
	p := &domain.Project{}
	err := r.pool.QueryRow(ctx,
		`SELECT id, name, organization_id, scm_url, scm_branch, scm_credential_id,
		        latest_revision, sync_status, last_sync_at, created_at, updated_at
		 FROM projects WHERE id = $1 AND organization_id = ANY($2)`,
		id, scope.OrgIDs,
	).Scan(&p.ID, &p.Name, &p.OrganizationID, &p.SCMURL, &p.SCMBranch,
		&p.SCMCredentialID, &p.LatestRevision, &p.SyncStatus, &p.LastSyncAt,
		&p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (r *ProjectRepo) Update(ctx context.Context, id uuid.UUID, input domain.ProjectCreate) (*domain.Project, error) {
	_, err := r.pool.Exec(ctx,
		`UPDATE projects SET scm_url = $2, scm_branch = $3, scm_credential_id = $4, updated_at = now()
		 WHERE id = $1`,
		id, input.SCMURL, input.SCMBranch, input.SCMCredentialID,
	)
	if err != nil {
		return nil, fmt.Errorf("update project: %w", err)
	}
	return r.GetByID(ctx, id)
}

func (r *ProjectRepo) UpdateScoped(ctx context.Context, id uuid.UUID, scope domain.Scope, input domain.ProjectCreate) (*domain.Project, error) {
	if scope.IsPlatformAdmin {
		return r.Update(ctx, id, input)
	}
	tag, err := r.pool.Exec(ctx,
		`UPDATE projects SET scm_url = $3, scm_branch = $4, scm_credential_id = $5, updated_at = now()
		 WHERE id = $1 AND organization_id = ANY($2)`,
		id, scope.OrgIDs, input.SCMURL, input.SCMBranch, input.SCMCredentialID,
	)
	if err != nil {
		return nil, fmt.Errorf("update project: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return nil, pgx.ErrNoRows
	}
	return r.GetByIDScoped(ctx, id, scope)
}

func (r *ProjectRepo) Delete(ctx context.Context, id uuid.UUID) error {
	_, err := r.pool.Exec(ctx, `DELETE FROM projects WHERE id = $1`, id)
	return err
}

func (r *ProjectRepo) DeleteScoped(ctx context.Context, id uuid.UUID, scope domain.Scope) error {
	if scope.IsPlatformAdmin {
		return r.Delete(ctx, id)
	}
	tag, err := r.pool.Exec(ctx,
		`DELETE FROM projects WHERE id = $1 AND organization_id = ANY($2)`,
		id, scope.OrgIDs,
	)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return nil
}

func (r *ProjectRepo) ListByOrg(ctx context.Context, orgID uuid.UUID) ([]domain.Project, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT id, name, organization_id, scm_url, scm_branch, scm_credential_id,
		        latest_revision, sync_status, last_sync_at, created_at, updated_at
		 FROM projects WHERE organization_id = $1 ORDER BY name`, orgID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var projects []domain.Project
	for rows.Next() {
		var p domain.Project
		if err := rows.Scan(&p.ID, &p.Name, &p.OrganizationID, &p.SCMURL, &p.SCMBranch,
			&p.SCMCredentialID, &p.LatestRevision, &p.SyncStatus, &p.LastSyncAt,
			&p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		projects = append(projects, p)
	}
	return projects, nil
}

// UpdateSyncStatus updates the sync status and revision of a project.
func (r *ProjectRepo) UpdateSyncStatus(ctx context.Context, id uuid.UUID, status, revision, artifactPath, errMsg string) error {
	var latestRevision *string
	if revision != "" {
		latestRevision = &revision
	}
	_, err := r.pool.Exec(ctx,
		`UPDATE projects SET
		        sync_status = $2,
		        latest_revision = COALESCE($3, latest_revision),
		        last_sync_at = now(),
		        updated_at = now()
		 WHERE id = $1`,
		id, status, latestRevision,
	)
	return err
}

// ─── Repositories ─────────────────────────────────────────────────────────────

// Repositories bundles all repository instances.
type Repositories struct {
	Organizations         *OrganizationRepo
	Users                 *UserRepo
	Teams                 *TeamRepo
	RoleAssignments       *RoleAssignmentRepo
	Projects              *ProjectRepo
	Credentials           *CredentialRepo
	Inventories           *InventoryRepo
	ExecutionEnvironments *ExecutionEnvironmentRepo
	JobTemplates          *JobTemplateRepo
	Jobs                  *JobRepo
	JobEvents             *JobEventRepo
	JobLogs               *JobLogRepo
}

// New creates all repositories.
func New(pool *Pool) *Repositories {
	return &Repositories{
		Organizations:         NewOrganizationRepo(pool),
		Users:                 NewUserRepo(pool),
		Teams:                 NewTeamRepo(pool),
		RoleAssignments:       NewRoleAssignmentRepo(pool),
		Projects:              NewProjectRepo(pool),
		Credentials:           NewCredentialRepo(pool),
		Inventories:           NewInventoryRepo(pool),
		ExecutionEnvironments: NewExecutionEnvironmentRepo(pool),
		JobTemplates:          NewJobTemplateRepo(pool),
		Jobs:                  NewJobRepo(pool),
		JobEvents:             NewJobEventRepo(pool),
		JobLogs:               NewJobLogRepo(pool),
	}
}
