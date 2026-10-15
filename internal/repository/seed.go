package repository

import (
	"context"
	"fmt"
	"log/slog"
	"os"

	"github.com/rverhe/aiwx/internal/domain"
)

// SeedDevAdmin creates a default organization and admin user if none exist.
// Credentials (dev only): admin@aiwx.local / admin
// Override password with SEED_ADMIN_PASSWORD.
func SeedDevAdmin(ctx context.Context, repos *Repositories, logger *slog.Logger) error {
	const defaultEmail = "admin@aiwx.local"

	_, err := repos.Users.GetByEmail(ctx, defaultEmail)
	if err == nil {
		return nil // already seeded
	}

	password := os.Getenv("SEED_ADMIN_PASSWORD")
	if password == "" {
		password = "admin"
	}

	org, err := repos.Organizations.Create(ctx, domain.OrganizationCreate{
		Name:        "Default",
		DisplayName: "Default Organization",
	})
	if err != nil {
		return fmt.Errorf("seed organization: %w", err)
	}

	user, err := repos.Users.Create(ctx, domain.UserCreate{
		Email:       defaultEmail,
		DisplayName: "Platform Admin",
		Password:    password,
	})
	if err != nil {
		return fmt.Errorf("seed user: %w", err)
	}

	userID := user.ID
	if _, err := repos.RoleAssignments.Create(ctx, domain.RoleAssignmentCreate{
		RoleName: domain.RolePlatformAdmin,
		UserID:   &userID,
	}, org.ID); err != nil {
		return fmt.Errorf("seed role assignment: %w", err)
	}

	logger.Info("seeded dev admin credentials",
		"email", defaultEmail,
		"organization", org.Name,
		"password", "from SEED_ADMIN_PASSWORD or 'admin'",
	)
	return nil
}
