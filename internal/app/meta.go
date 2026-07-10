package app

// PageMeta holds all SEO and Open Graph metadata for a page.
// Injected as data["Meta"] in every handler.
// Rendered by {{template "page-head" .}} in every template.
//
// Two constructors cover all cases:
//   app.DefaultMeta(...)  — public pages, full SEO + OG + Twitter
//   app.AuthMeta(...)     — authenticated pages, noindex, no OG
//
// For pages with dynamic OG images (invoices, business profiles):
//   use DefaultMeta then set .OGImage explicitly.
type PageMeta struct {
	// Title is the full browser tab title.
	// Public convention:  "Page Name | PSILOCONVALLEY"
	// Homepage:           "PSILOCONVALLEY — Tagline"
	// Always required.
	Title string

	// Description is the meta description and og:description.
	// Keep under 160 characters for search results.
	// Public pages only.
	Description string

	// TwitterDesc is the twitter:description value.
	// Falls back to Description in the template if empty.
	// Useful when Twitter copy should differ from meta description.
	TwitterDesc string

	// Canonical is the full canonical URL including https://
	// Example: "https://psiloconvalley.com/pricing"
	// Public pages only.
	Canonical string

	// OGImage is the full URL to the Open Graph image.
	// Always rendered at 1200x630.
	// Defaults to /og/default.jpg when empty.
	// Override for invoice pages: /og/invoice/{id}.jpg
	// Override for profile pages: business logo URL if set.
	OGImage string

	// Robots controls the meta robots directive.
	// Public pages:        "index,follow"
	// Authenticated pages: "noindex,nofollow"
	// Set automatically by constructors — rarely override manually.
	Robots string

	// IsPublic controls whether the full OG + Twitter block renders.
	// true  → full SEO meta, OG tags, Twitter card, canonical
	// false → title, viewport, charset, favicon, noindex only
	// Set automatically by constructors.
	IsPublic bool

	// SkipTitle tells page-head not to render <title>.
	// Use for templates that build their own dynamic title from i18n keys.
	// Example: invoice_new.tmpl builds title from .DocumentType + .Mode
	SkipTitle bool
}

const (
	// metaDefaultOGImage is the fallback OG image for all pages.
	metaDefaultOGImage = "https://psiloconvalley.com/og/default.jpg"
)

// DefaultMeta constructs a PageMeta for public-facing pages.
// Renders full SEO meta, Open Graph, and Twitter Card tags.
// robots = index,follow.
//
// Use for: home, pricing, research, tools, enterprise, security,
//          public profile pages, public invoice pages.
func DefaultMeta(title, description, twitterDesc, canonical string) PageMeta {
	return PageMeta{
		Title:       title,
		Description: description,
		TwitterDesc: twitterDesc,
		Canonical:   canonical,
		OGImage:     metaDefaultOGImage,
		Robots:      "index,follow",
		IsPublic:    true,
	}
}

// AuthMeta constructs a PageMeta for authenticated app pages.
// Renders only title, viewport, charset, and favicon.
// robots = noindex,nofollow — authenticated pages must not appear in search.
//
// Use for: dashboard, invoices, clients, estimates, expenses,
//          reports, profile, recurring, admin, login, register.
func AuthMeta(title string) PageMeta {
	return PageMeta{
		Title:    title,
		Robots:   "noindex,nofollow",
		IsPublic: false,
	}
}
