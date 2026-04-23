package main

import (
	"fmt"
	"log"
	"net/http"
)

// Handler function for the home page
func homePage(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "<h1>Welcome to Psilocon Valley</h1>")
	fmt.Fprintf(w, "<p>This site is built with Go and hosted on Railway.</p>")
}

// Handler function for the blog (placeholder for now)
func blogPage(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "<h1>The Blog</h1>")
	fmt.Fprintf(w, "<p>Coming soon...</p>")
}

func main() {
	// Register routes
	http.HandleFunc("/", homePage)       // When visiting root "/"
	http.HandleFunc("/blog", blogPage)   // When visiting "/blog"

	fmt.Println("🚀 Server starting on http://localhost:8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}
