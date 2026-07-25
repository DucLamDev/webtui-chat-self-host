package domain

import (
	"errors"
	"time"
)

var (
	ErrAutomationConfigInvalid        = errors.New("automation config khong hop le")
	ErrAutomationInstallationConflict = errors.New("automation installation da ton tai")
	ErrAutomationInstallationNotFound = errors.New("khong tim thay automation installation")
	ErrAutomationTemplateNotFound     = errors.New("khong tim thay automation template")
	ErrDomainAlreadyClaimed           = errors.New("domain da duoc claim")
	ErrDomainClaimNotFound            = errors.New("khong tim thay domain claim")
	ErrDomainInvalid                  = errors.New("domain khong hop le")
	ErrDomainIsPrimary                = errors.New("khong the xoa primary domain")
	ErrDomainVerificationExpired      = errors.New("domain verification da het han")
	ErrDomainVerificationFailed       = errors.New("domain verification that bai")
	ErrDeploymentRequestConflict      = errors.New("deployment request da ton tai")
	ErrOIDCProviderConflict           = errors.New("OIDC provider da ton tai")
	ErrOIDCProviderNotFound           = errors.New("khong tim thay OIDC provider")
	ErrZoneQuotaExceeded              = errors.New("zone da vuot quota")
	ErrWorkspaceZoneMismatch          = errors.New("workspace khong thuoc zone")
	ErrZoneAccessDenied               = errors.New("khong co quyen quan ly zone")
	ErrZoneNotFound                   = errors.New("khong tim thay zone")
)

type Zone struct {
	ID                 string
	Slug               string
	Name               string
	Kind               string
	Status             string
	RegistrationMode   string
	PrimaryWorkspaceID *string
	Metadata           map[string]any
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

type DeploymentRequest struct {
	ID                    string
	ZoneID                string
	RequestedMode         string
	RequestedDatabaseMode string
	Status                string
	IdempotencyKey        string
	RequestedBy           *string
	FailureReason         *string
	Metadata              map[string]any
	CreatedAt             time.Time
	UpdatedAt             time.Time
	CompletedAt           *time.Time
}

type ZoneAdminOverview struct {
	Zone        Zone
	Domains     []Domain
	Deployments []Deployment
}

type ZoneQuota struct {
	ZoneID                     string
	MaxWorkspaces              int
	MaxMembers                 int
	MaxStorageBytes            int64
	MaxAutomationInstallations int
	MaxWebhooks                int
	EnforcementMode            string
	CreatedAt                  time.Time
	UpdatedAt                  time.Time
}

type ZoneUsage struct {
	Workspaces              int
	Members                 int
	StorageBytes            int64
	AutomationInstallations int
	Webhooks                int
}

type OIDCProvider struct {
	ID                   string
	ZoneID               string
	Name                 string
	IssuerURL            string
	ClientID             string
	ClientSecretRef      *string
	Scopes               []string
	ClaimMapping         map[string]any
	JITProvisioning      bool
	RequireVerifiedEmail bool
	Status               string
	CreatedBy            *string
	CreatedAt            time.Time
	UpdatedAt            time.Time
}

type Domain struct {
	ID                    string
	ZoneID                string
	Domain                string
	Kind                  string
	Status                string
	VerificationMethod    string
	VerificationToken     string
	VerifiedAt            *time.Time
	VerificationExpiresAt *time.Time
	VerificationAttempts  int
	LastVerificationError *string
	TLSStatus             string
	LastCheckedAt         *time.Time
	CreatedAt             time.Time
	UpdatedAt             time.Time
}

type Deployment struct {
	ID            string
	ZoneID        string
	Mode          string
	WebBaseURL    string
	APIBaseURL    string
	WSBaseURL     string
	AdminBaseURL  *string
	DatabaseMode  string
	StorageBucket *string
	RedisPrefix   *string
	Status        string
	Metadata      map[string]any
}

type WorkspaceRef struct {
	ID   string
	Slug string
	Name string
}

type ResolvedZone struct {
	Zone       Zone
	Domain     Domain
	Deployment *Deployment
	Workspace  *WorkspaceRef
}

type DomainClaim struct {
	Zone      Zone
	Domain    Domain
	Workspace *WorkspaceRef
}

type AutomationTemplate struct {
	ID             string
	Key            string
	Name           string
	Description    *string
	ZoneKind       string
	TemplateType   string
	RuntimeKind    string
	ConfigSchema   map[string]any
	DefaultConfig  map[string]any
	RequiredScopes []string
	Status         string
	CreatedAt      time.Time
	UpdatedAt      time.Time
}

type AutomationInstallation struct {
	ID               string
	ZoneID           string
	WorkspaceID      *string
	TemplateID       *string
	TemplateKey      *string
	Name             string
	Status           string
	Config           map[string]any
	SecretRef        *string
	RuntimeWebhookID *string
	InstalledBy      *string
	CreatedAt        time.Time
	UpdatedAt        time.Time
}
