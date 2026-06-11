package repo

import (
	"strings"
	"time"

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
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

func (u *User) CheckPassword(plain string) bool {
	if u.PasswordHash == "" {
		return false
	}
	return bcrypt.CompareHashAndPassword(
		[]byte(u.PasswordHash),
		[]byte(plain),
	) == nil
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
