package main

import (
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
)

func main() {
	// Fetch all necessary js
	if _, err := os.Stat("assets/js/htmx.min.js"); errors.Is(err, os.ErrNotExist) {
		log.Println("HTMX lib doesn't exist, fetching...")
	}
	if err := os.MkdirAll("assets/js", os.ModePerm); err != nil {
		log.Fatalf("could not create assets/js dir: %v", err)
	}
	h, err := os.Create("assets/js/htmx.min.js")
	if err != nil {
		log.Fatalf("could not create htmx.min.js file: %v", err)
	}
	resp, err := http.Get("https://unpkg.com/htmx.org/dist/htmx.min.js")
	if _, err := io.Copy(h, resp.Body); err != nil {
		log.Fatalf("could not download htmx.min.js file: %v", err)

	}
	defer resp.Body.Close()
	defer h.Close()

	if _, err := os.Stat("assets/js/Readability.js"); errors.Is(err, os.ErrNotExist) {
		log.Println("Readability.js lib doesn't exist, fetching...")
	}
	r, err := os.Create("assets/js/Readability.js")
	if err != nil {
		log.Fatalf("could not create Readability.js file: %v", err)
	}
	resp2, err := http.Get("https://raw.githubusercontent.com/mozilla/readability/main/Readability.js")
	if _, err := io.Copy(r, resp2.Body); err != nil {
		log.Fatalf("could not download Readability.js file: %v", err)

	}
	defer resp2.Body.Close()
	defer r.Close()

	// Check if the tailwindcss standalone executable is on the present $PATH
	_, err = exec.LookPath("tailwindcss")
	if err != nil {
		log.Fatalf("no tailwind standalone executable found, please install one. See: %s. Exiting with error: %v",
			"https://tailwindcss.com/blog/standalone-cli",
			err)
	}

	_, err = exec.LookPath("playwright")
	if err != nil {
		cmd := exec.Command("sh", "-c", "go install github.com/playwright-community/playwright-go/cmd/playwright@latest && playwright install --with-deps")
		log.Println("installing playwright and its drivers...")
		log.Println("hang tight, it can take a while")
		if err := cmd.Run(); err != nil {
			log.Fatalf("error executing playwright installation: %v", err)
		}
	}
	_, err = exec.LookPath("templ")
	if err != nil {
		cmd := exec.Command("sh", "-c", "go install github.com/a-h/templ/cmd/templ@latest")
		log.Println("installing templ...")
		if err := cmd.Run(); err != nil {
			log.Fatalf("error executing templ installation: %v", err)
		}
	}
	_, err = exec.LookPath("air")
	if err != nil {
		cmd := exec.Command("sh", "-c", "go install github.com/cosmtrek/air@latest")
		log.Println("installing air...")
		if err := cmd.Run(); err != nil {
			log.Fatalf("error executing air installation: %v", err)
		}
	}
	log.Println("Bootstrap complete!")
}
