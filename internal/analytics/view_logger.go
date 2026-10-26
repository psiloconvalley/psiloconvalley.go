package analytics

// ViewerType classifies the source of an invoice view.
const (
	ViewerPublicLink = "public_link"
	ViewerBot        = "bot"
)

// Decision is the output of DecideView. It tells the caller whether to
// write a row and, if so, how to classify the viewer.
type Decision struct {
	ShouldLog  bool
	ViewerType string
}

// DecideView is pure. It takes no *http.Request, no DB, no clock.
// All inputs are explicit so the function is trivially testable.
//
// Rules, in order:
//  1. Owner (authenticated session) → do not log.
//  2. Anonymous owner (cookie token matches invoice.anonymous_token) → do not log.
//  3. Bot user agent → log as "bot".
//  4. Anything else → log as "public_link".
//
// By the time we reach this function, the handler has already validated
// that the caller is authorized to view the invoice (owner session, anon
// cookie match, or valid ?access=TOKEN). We do not re-check authorization
// here; we only classify.
func DecideView(isOwner, isAnonTokenMatch bool, userAgent string) Decision {
	if isOwner || isAnonTokenMatch {
		return Decision{ShouldLog: false}
	}
	if isBot(userAgent) {
		return Decision{ShouldLog: true, ViewerType: ViewerBot}
	}
	return Decision{ShouldLog: true, ViewerType: ViewerPublicLink}
}
