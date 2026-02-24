package initmod

import (
	"context"
	"fmt"
	"strings"

	"github.com/leeforge/core/core"
	"github.com/leeforge/core/server/config"
	"github.com/leeforge/core/server/ent"
	"github.com/leeforge/core/server/ent/domain"
	"github.com/leeforge/core/server/ent/domainmembership"
	"github.com/leeforge/core/server/ent/domaintype"
	"github.com/leeforge/core/server/ent/systemconfig"
	"github.com/leeforge/core/server/ent/user"
	"github.com/leeforge/core/server/pkg/crypto"

	"github.com/leeforge/framework/logging"
)

const (
	SystemInitializedKey = "system.initialized"
	platformTenantID     = "platform"
)

type InitService struct {
	client        *ent.Client
	config        *config.Config
	logger        logging.Logger
	domainService core.DomainResolver
}

func NewInitService(client *ent.Client, config *config.Config, logger logging.Logger, domainService core.DomainResolver) *InitService {
	return &InitService{
		client:        client,
		config:        config,
		logger:        logger,
		domainService: domainService,
	}
}

// CheckStatus checks if the system is already initialized
func (s *InitService) CheckStatus(ctx context.Context) (*InitStatusResponse, error) {
	// Check SystemConfig table
	initialized := false

	// Check if the initialization flag exists
	exist, err := s.client.SystemConfig.Query().
		Where(systemconfig.KeyEQ(SystemInitializedKey)).
		Exist(ctx)
	if err != nil {
		// If table doesn't exist or other db error, return error
		// But for first run, table should exist if migration ran
		return nil, fmt.Errorf("failed to check system config: %w", err)
	}

	if exist {
		config, err := s.client.SystemConfig.Query().
			Where(systemconfig.KeyEQ(SystemInitializedKey)).
			Only(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to get system config: %w", err)
		}
		if config.Value == "true" {
			initialized = true
		}
	}

	return &InitStatusResponse{
		Initialized: initialized,
		Version:     "1.0.0",
	}, nil
}

// Initialize performs the system initialization
func (s *InitService) Initialize(ctx context.Context, req *InitSetupRequest) (*InitSetupResponse, error) {
	// 1. Check if already initialized
	status, err := s.CheckStatus(ctx)
	if err != nil {
		return nil, err
	}
	if status.Initialized {
		return nil, fmt.Errorf("system already initialized")
	}

	// 2. Verify init key
	if req.InitKey != s.config.Init.SecretKey {
		return nil, fmt.Errorf("invalid initialization key")
	}

	// 3. Start transaction
	tx, err := s.client.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to start transaction: %w", err)
	}

	// Ensure rollback on panic or error
	defer func() {
		if v := recover(); v != nil {
			tx.Rollback()
			panic(v)
		}
	}()

	// 4. Create platform super admin user
	adminUser, err := s.createAdminUser(ctx, tx, req.Admin)
	if err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to create admin user: %w", err)
	}

	// 5. Initialize platform domain and assign admin membership
	if err := s.ensurePlatformDomain(ctx, tx, adminUser); err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to initialize platform domain: %w", err)
	}

	// 6. Mark system as initialized
	err = s.markAsInitialized(ctx, tx)
	if err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to mark system as initialized: %w", err)
	}

	// 9. Commit transaction
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return &InitSetupResponse{
		AdminUser: &UserInfo{
			ID:       adminUser.ID.String(),
			Username: adminUser.Username,
			Email:    adminUser.Email,
		},
		RolesCreated:       0,
		PermissionsCreated: 0, // Placeholder
		Message:            "System initialized successfully",
	}, nil
}

func (s *InitService) createAdminUser(ctx context.Context, tx *ent.Tx, input AdminInput) (*ent.User, error) {
	// Check if user exists
	exist, err := tx.User.Query().
		Where(user.UsernameEQ(input.Username)).
		Exist(ctx)
	if err != nil {
		return nil, err
	}
	if exist {
		return nil, fmt.Errorf("user %s already exists", input.Username)
	}

	// Hash password
	hashedPassword, err := crypto.HashPassword(input.Password)
	if err != nil {
		return nil, err
	}

	// Create user
	return tx.User.Create().
		SetUsername(input.Username).
		SetEmail(input.Email).
		SetPasswordHash(hashedPassword).
		SetNickname(input.Nickname).
		SetStatus(user.StatusActive).
		SetIsSuperAdmin(true).
		Save(ctx)
}

func (s *InitService) markAsInitialized(ctx context.Context, tx *ent.Tx) error {
	// Check if config exists
	exist, err := tx.SystemConfig.Query().
		Where(systemconfig.KeyEQ(SystemInitializedKey)).
		Exist(ctx)
	if err != nil {
		return err
	}

	if exist {
		// Update
		_, err = tx.SystemConfig.Update().
			Where(systemconfig.KeyEQ(SystemInitializedKey)).
			SetValue("true").
			Save(ctx)
		return err
	}

	// Create
	_, err = tx.SystemConfig.Create().
		SetKey(SystemInitializedKey).
		SetValue("true").
		SetDescription("System initialization status").
		Save(ctx)
	return err
}

// ensurePlatformDomain creates the platform domain type, root domain, and admin membership.
// It gracefully skips if domainService is nil (backward compatibility).
func (s *InitService) ensurePlatformDomain(ctx context.Context, tx *ent.Tx, adminUser *ent.User) error {
	if s.domainService == nil {
		s.logger.Info("Domain service not available, skipping domain initialization")
		return nil
	}

	// 1. Ensure DomainType: platform
	_, err := tx.DomainType.Create().
		SetCode("platform").
		SetDisplayName("Platform").
		SetStatus(domaintype.StatusActive).
		Save(ctx)
	if err != nil {
		// Might already exist, ignore unique constraint errors
		if !strings.Contains(err.Error(), "unique") && !strings.Contains(err.Error(), "duplicate") {
			return fmt.Errorf("create platform domain type: %w", err)
		}
	}

	// 2. Ensure Domain: platform:root
	d, err := tx.Domain.Create().
		SetTypeCode("platform").
		SetKey("root").
		SetDisplayName("Platform Root").
		SetStatus(domain.StatusActive).
		Save(ctx)
	if err != nil {
		if !strings.Contains(err.Error(), "unique") && !strings.Contains(err.Error(), "duplicate") {
			return fmt.Errorf("create platform root domain: %w", err)
		}
		// Query existing
		d, err = tx.Domain.Query().
			Where(domain.TypeCodeEQ("platform"), domain.KeyEQ("root")).
			Only(ctx)
		if err != nil {
			return fmt.Errorf("query existing platform root domain: %w", err)
		}
	}

	// 3. Create DomainMembership for admin
	_, err = tx.DomainMembership.Create().
		SetDomainID(d.ID).
		SetSubjectType(domainmembership.SubjectTypeUser).
		SetSubjectID(adminUser.ID).
		SetMemberRole("owner").
		SetIsDefault(true).
		SetStatus(domainmembership.StatusActive).
		Save(ctx)
	if err != nil {
		if !strings.Contains(err.Error(), "unique") && !strings.Contains(err.Error(), "duplicate") {
			return fmt.Errorf("create admin domain membership: %w", err)
		}
	}

	s.logger.Info("Platform domain initialized successfully")
	return nil
}
