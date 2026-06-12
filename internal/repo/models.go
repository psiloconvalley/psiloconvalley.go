package repo

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/argon2"
	"golang.org/x/crypto/bcrypt"
)

type BusinessProfile struct {
	ID        int64
	UserID    int64
	Name      string
	Email     string
	Address   string
	City      string
	State     string
	Zip       string
	Country   string
	TaxID     string
	Currency  string
	LogoURL   string
	CreatedAt time.Time
}

type Client struct {
	ID                int64
	BusinessProfileID int64
	Name              string
	Email             string
	Address           string
	City              string
	State             string
	Zip               string
	Country           string
	Phone             string
	Notes             string
	DefaultTaxRateBps int64
	PaymentTerms      string
	CreatedAt         time.Time
}

type User struct {
	ID               int64
	Email            string
	PasswordHash     string
	Plan             string
	Provider         string
	GoogleID         string
	Name             string
	AvatarURL        string
	StripeCustomerID string
	StripeConnectID  string
	NextInvoiceSeq   int
	NextEstimateSeq  int
	Language         string
	PasswordAlgo        string
	MagicToken          string
	MagicTokenExpiresAt *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
	FailedLoginAttempts int
	LockedUntil         *time.Time
}

// Argon2id parameters — OWASP recommended minimum
const (
	argonTime    = 3
	argonMemory  = 64 * 1024 // 64 MB
	argonThreads = 4
	argonKeyLen  = 32
	argonSaltLen = 16
)

// HashPassword creates an Argon2id hash. All new passwords use this.
func HashPassword(plain string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generate salt: %w", err)
	}
	key := argon2.IDKey([]byte(plain), salt, argonTime, argonMemory, argonThreads, argonKeyLen)
	return fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	), nil
}

// CheckPassword verifies a password against the stored hash.
// Supports both bcrypt (legacy) and Argon2id (current).
func (u *User) CheckPassword(plain string) bool {
	if u.PasswordHash == "" {
		return false
	}
	if u.PasswordAlgo == "argon2id" || strings.HasPrefix(u.PasswordHash, "$argon2id$") {
		return verifyArgon2id(plain, u.PasswordHash)
	}
	return bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(plain)) == nil
}

// NeedsRehash returns true if the password is still using bcrypt.
func (u *User) NeedsRehash() bool {
	return u.PasswordAlgo != "argon2id"
}
// IsLocked returns true if the account is currently locked out.
func (u *User) IsLocked() bool {
	if u.LockedUntil == nil {
		return false
	}
	return time.Now().Before(*u.LockedUntil)
}

// LockoutRemaining returns how many minutes until the lockout expires.
func (u *User) LockoutRemaining() int {
	if u.LockedUntil == nil {
		return 0
	}
	remaining := time.Until(*u.LockedUntil)
	if remaining <= 0 {
		return 0
	}
	return int(remaining.Minutes()) + 1
}
func verifyArgon2id(plain, encoded string) bool {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 {
		return false
	}
	var version int
	var memory, time uint32
	var threads uint8
	_, _ = fmt.Sscanf(parts[2], "v=%d", &version)
	_, _ = fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &memory, &time, &threads)
	salt, err := base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return false
	}
	expectedKey, err := base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return false
	}
	key := argon2.IDKey([]byte(plain), salt, time, memory, threads, uint32(len(expectedKey)))
	return subtle.ConstantTimeCompare(key, expectedKey) == 1
}

func (u *User) IsGoogleUser() bool {
	return u.Provider == "google" || u.GoogleID != ""
}

func (u *User) DisplayName() string {
	if u.Name != "" {
		return u.Name
	}
	parts := strings.SplitN(u.Email, "@", 2)
	return parts[0]
}

type Invoice struct {
	ID                int64
	BusinessProfileID *int64
	ClientID          *int64
	UserID            *int64
	AnonymousToken    string
	LogoURL           string
	CompanyName       string
	CompanyEmail      string
	CompanyAddress    string
	CompanyCity       string
	CompanyZip        string
	CompanyState      string
	CompanyCountry    string
	ClientName        string
	ClientEmail       string
	ClientAddress     string
	ClientCity        string
	ClientZip         string
	ClientState       string
	ClientCountry     string
	InvoiceNumber       string
	IssueDate           time.Time
	DueDate             *time.Time
	TaxRateBps          int64
	DiscountAmountCents int64
	Notes               string
	PaymentDetails      string
	SubtotalCents       int64
	TaxAmountCents      int64
	TotalCents          int64
	Currency            string
	Status              string
	DocumentType        string
	CreatedAt           time.Time
	UpdatedAt           time.Time
	ShowLogo            bool
	ShowTitle           bool
	AutoReminders       bool
	TemplateID          string
	BrandColor          string
	LogoPosition        string
	PublicToken         string
}

type InvoiceItem struct {
	ID             int64
	InvoiceID      int64
	Description    string
	Details        string
	Quantity       float64
	UnitPriceCents int64
	LineTotalCents int64
}

type EstimateResponse struct {
	ID         int64
	EstimateID int64
	Action     string
	Message    string
	ClientName string
	CreatedAt  time.Time
}

type DashboardStats struct {
	RevenueCents     int64
	OutstandingCents int64
	OverdueCents     int64
	MonthlyCount     int64
	TotalCount       int64
}

type AdminStats struct {
	TotalUsers            int
	NewUsersThisWeek      int
	ProUsers              int
	TotalInvoices         int
	TotalEstimates        int
	EstimatesSent         int
	EstimatesAccepted     int
	EstimatesDeclined     int
	EstimatesConverted    int
	TotalRevenueCents     int64
	TotalOutstandingCents int64
	MonthlyInvoices       int
}

type InvoiceReportRow struct {
	InvoiceNumber string
	ClientName    string
	ClientEmail   string
	IssueDate     time.Time
	DueDate       *time.Time
	Status        string
	SubtotalCents int64
	TaxCents      int64
	TotalCents    int64
	Currency      string
	DaysToPayment *int
}

type ClientScorecard struct {
	ClientName       string
	TotalBilled      int64
	TotalPaid        int64
	Outstanding      int64
	Overdue          int64
	InvoiceCount     int
	PaidCount        int
	OverdueCount     int
	AvgDaysToPayment int
	OnTimeRate       int
	Score            int
}
