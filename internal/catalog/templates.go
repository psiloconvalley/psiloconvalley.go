package catalog

// InvoiceTemplate defines an available invoice layout.
// Templates are static — no database table. Adding a new template
// means adding an entry here and a corresponding .tmpl file.
type InvoiceTemplate struct {
	ID          string
	Name        string
	Description string
	ProOnly     bool
}

// InvoiceTemplates is the authoritative list of available templates.
// The handler validates template_id against this list on save.
// The invoice form reads this list to render the template picker.
var InvoiceTemplates = []InvoiceTemplate{
	{ID: "classic", Name: "Classic", Description: "Deep slate accents, professional layout", ProOnly: false},
	{ID: "minimal", Name: "Minimal", Description: "Clean lines, light grey, modern feel", ProOnly: true},
	{ID: "bold", Name: "Bold", Description: "High contrast header, strong typography", ProOnly: true},
}

const DefaultTemplateID = "classic"
const DefaultBrandColor = "#0d1422"

// ValidTemplateID returns true if the given ID matches a known template.
func ValidTemplateID(id string) bool {
	for _, t := range InvoiceTemplates {
		if t.ID == id {
			return true
		}
	}
	return false
}

// TemplateRequiresPro returns true if the given template is Pro-only.
// Returns true for unknown IDs as a safe default.
func TemplateRequiresPro(id string) bool {
	for _, t := range InvoiceTemplates {
		if t.ID == id {
			return t.ProOnly
		}
	}
	return true
}
