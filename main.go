package main

import (
	"context"
	"database/sql"
	"embed"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"

	"github.com/a-h/templ"
	"github.com/playwright-community/playwright-go"
	_ "modernc.org/sqlite"
)

var (
	//go:embed css assets
	FS embed.FS

	db *sql.DB

	articleList    []Article
	currentArticle Article

	wg sync.WaitGroup
)

func initDatabase(dbPath string) {
	var err error
	db, err = sql.Open("sqlite", dbPath)
	if err != nil {
		log.Fatalf("could not init db: %v", err)
	}
	_, err = db.ExecContext(context.Background(),
		`CREATE TABLE IF NOT EXISTS article (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			html TEXT,
			url TEXT NOT NULL,
			title TEXT NOT NULL
	)`)
	if err != nil {
		log.Fatalf("could not create articles table: %v", err)
	}
}

func getArticleHTML(url string) {
	pw, err := playwright.Run()
	if err != nil {
		log.Fatalf("could not start playwright: %v", err)
	}
	browser, err := pw.Firefox.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(true),
	})
	if err != nil {
		log.Fatalf("could not launch browser: %v", err)
	}
	page, err := browser.NewPage()
	if err != nil {
		log.Fatalf("could not create page: %v", err)
	}
	if _, err = page.Goto(url); err != nil {
		log.Fatalf("could not goto: %v", err)
	}
	_ = page.WaitForLoadState(playwright.PageWaitForLoadStateOptions{State: playwright.LoadStateDomcontentloaded})
	path := "http://localhost:3333/public/assets/js/Readability.js"
	if _, err = page.AddScriptTag(playwright.PageAddScriptTagOptions{URL: &path}); err != nil {
		log.Fatalf("could not inject Readability script %v", err)
	}
	html, err := page.Evaluate("() => { let article = new Readability(document).parse(); return article.content; }")
	if err != nil {
		log.Fatalf("could not get text content: %v", err)
	}
	if len(html.(string)) > 0 {
		article, err := getArticleByUrl(url)
		if err != nil {
			log.Fatalf("could not get article with url %s: %v", url, err)
		}
		if _, err := updateArticleHTML(int64(article.ID), html.(string)); err != nil {
			log.Fatalf("could not update article %d with content %s: %v", article.ID, html, err)
		}
		currentArticle = Article{Title: article.Title, Url: article.Url, HTML: html.(string)}
	}
	if err = browser.Close(); err != nil {
		log.Fatalf("could not close browser: %v", err)
	}
	if err = pw.Stop(); err != nil {
		log.Fatalf("could not stop Playwright: %v", err)
	}
	defer wg.Done()
}

func getArticleLinks() {
	pw, err := playwright.Run()
	if err != nil {
		log.Fatalf("could not start playwright: %v", err)
	}
	browser, err := pw.Firefox.Launch(playwright.BrowserTypeLaunchOptions{
		Headless: playwright.Bool(true),
	})
	if err != nil {
		log.Fatalf("could not launch browser: %v", err)
	}
	page, err := browser.NewPage()
	if err != nil {
		log.Fatalf("could not create page: %v", err)
	}
	if _, err = page.Goto("https://sporza.be/nl/pas-verschenen/"); err != nil {
		log.Fatalf("could not goto: %v", err)
	}
	entries, err := page.Locator(".sw-card-module-card").All()
	if err != nil {
		log.Fatalf("could not get entries: %v", err)
	}
	for _, entry := range entries {
		titleEntry := entry.Locator(".sw-title-module-title").First()
		title, err := titleEntry.TextContent()
		link, err := entry.GetAttribute("href")
		if err != nil {
			log.Fatalf("could not get text content: %v", err)
		}
		articleList = append(articleList, Article{Title: title, Url: link})
	}
	if err = browser.Close(); err != nil {
		log.Fatalf("could not close browser: %v", err)
	}
	if err = pw.Stop(); err != nil {
		log.Fatalf("could not stop Playwright: %v", err)
	}
	defer wg.Done()
}

type Article struct {
	HTML  string `json:"html,omitempty"`
	Url   string `json:"url,omitempty"`
	Title string `json:"title,omitempty"`
}

type ArticleDbRow struct {
	ID int
	Article
}

func addArticle(a *Article) (int64, error) {
	result, _ := db.ExecContext(
		context.Background(),
		`INSERT INTO article (html, url, title) VALUES (?, ?, ?);`,
		a.HTML,
		a.Url,
		a.Title)
	return result.LastInsertId()
}

func updateArticleHTML(articleId int64, html string) (int64, error) {
	result, _ := db.ExecContext(context.Background(),
		`UPDATE article
		 SET html = $2
		 WHERE id = $1;`,
		articleId, html)
	return result.RowsAffected()
}

func getArticleById(id int64) (ArticleDbRow, error) {
	var article ArticleDbRow
	row := db.QueryRow(`SELECT * FROM article WHERE id=?`, id)
	err := row.Scan(&article.ID, &article.HTML, &article.Url, &article.Title)
	if err != nil {
		return ArticleDbRow{}, err
	}
	return article, nil
}

func getArticleByUrl(url string) (ArticleDbRow, error) {
	var article ArticleDbRow
	row := db.QueryRow(`SELECT * FROM article WHERE url=?`, url)
	err := row.Scan(&article.ID, &article.HTML, &article.Url, &article.Title)
	if err != nil {
		return ArticleDbRow{}, err
	}
	return article, nil
}

func getArticlesCount() (int64, error) {
	var count int64
	err := db.QueryRow("SELECT COUNT(*) FROM article").Scan(&count)
	if err != nil {
		return 0, err
	} else {
		return count, nil
	}
}

func getArticles() ([]*ArticleDbRow, error) {
	var articles []*ArticleDbRow
	rows, err := db.Query("SELECT * FROM article")
	defer rows.Close()
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		article := new(ArticleDbRow)
		if err := rows.Scan(&article.ID, &article.HTML, &article.Url, &article.Title); err != nil {
			return nil, err
		}
		articles = append(articles, article)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return articles, nil
}

func deleteArticleById(id int64) (int64, error) {
	result, _ := db.ExecContext(context.Background(), "DELETE FROM article WHERE id = $1", id)
	return result.RowsAffected()
}

func parseHtml(html string) templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) (err error) {
		if _, err := io.WriteString(w, html); err != nil {
			return err
		}
		return nil
	})
}

func main() {
	dbPath := os.Getenv("SQLITE_DB_PATH")
	if len(dbPath) == 0 {
		log.Fatalf("please specifiy the SQLITE_DB_PATH env var")
	}
	initDatabase(dbPath)
	if db == nil {
		log.Fatalf("Db init failed")
	}
	if err := db.Ping(); err != nil {
		log.Fatalf("error setting up db connection, ping failed: %v", err)
	}
	log.Println("Db init succesful")
	count, err := getArticlesCount()
	if err != nil {
		log.Fatalf("could not query row count for the articles table: %v", err)
	}
	if count == 0 {
		wg.Add(1)
		go getArticleLinks()
		wg.Wait()
		for _, article := range articleList {
			_, err := addArticle(&article)
			if err != nil {
				log.Fatalf("could not add article: %v", err)
			}
		}
	} else {
		articles, err := getArticles()
		if err != nil {
			log.Fatalf("could not get articles from db: %v", err)
		}
		for _, article := range articles {
			articleList = append(articleList, Article{Url: article.Url, HTML: article.HTML, Title: article.Title})
		}

	}
	fetchHTMLContent := func(w http.ResponseWriter, r *http.Request) {
		url := r.URL.Query().Get("url")
		wg.Add(1)
		go getArticleHTML(url)
		wg.Wait()
		c := article(parseHtml(currentArticle.HTML))
		c.Render(context.Background(), w)

	}
	http.Handle("/", templ.Handler(page(articleList)))
	http.HandleFunc("/get-article", fetchHTMLContent)
	http.Handle("/get-articles", templ.Handler(articleListComponent(articleList)))
	http.Handle("/public/", http.StripPrefix("/public", http.FileServer(http.FS(FS))))
	fmt.Println("Listening on port 3333")
	http.ListenAndServe(":3333", nil)
}
