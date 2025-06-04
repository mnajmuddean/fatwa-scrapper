package main

import (
	"compress/gzip"
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/PuerkitoBio/goquery"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
	"github.com/robfig/cron/v3"
)

type Fatwa struct {
	ID       int
	Title    string
	URL      string
	Date     string
	Hits     int
	Category string
	Content  string
}
type FatwaBot struct {
	bot    *tgbotapi.BotAPI
	fatwas []Fatwa
}

func main() {
	// Create a new cron scheduler
	c := cron.New()

	// Schedule to run at 3:00 AM on the last day of every month
	_, err := c.AddFunc("0 3 28-31 * *", func() {
		if isLastDayOfMonth() {
			log.Println("Running monthly scraping job...")
			singlePageScraping()
		}
	})

	if err != nil {
		log.Fatal("Error scheduling cron job:", err)
	}

	// Start the cron scheduler
	c.Start()
	defer c.Stop() // Ensure cron stops when main exits

	// Load environment variables from .env file
	err = godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file")
	}

	// Get the token
	botToken := os.Getenv("BOT_TOKEN")
	if botToken == "" {
		log.Fatal("BOT_TOKEN not set in environment")
	}

	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Panic(err)
	}

	bot.Debug = true
	log.Printf("Authorized on account %s", bot.Self.UserName)

	// Load fatwa data from CSV
	fatwas, err := loadFatwaData("fatwa.csv")
	if err != nil {
		log.Fatalf("Error loading fatwa data: %v", err)
	}

	fatwaBot := &FatwaBot{
		bot:    bot,
		fatwas: fatwas,
	}

	log.Printf("Loaded %d fatwas", len(fatwas))

	// Start bot in a goroutine
	go fatwaBot.start()

	// Wait for interrupt signal to gracefully shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")
}

func (fb *FatwaBot) start() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := fb.bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message != nil {
			fb.handleMessage(update.Message)
		} else if update.CallbackQuery != nil {
			fb.handleCallbackQuery(update.CallbackQuery)
		}
	}
}

func (fb *FatwaBot) handleMessage(message *tgbotapi.Message) {
	chatID := message.Chat.ID
	text := message.Text

	switch {
	case text == "/start":
		fb.sendWelcomeMessage(chatID)
	case text == "/help":
		fb.sendHelpMessage(chatID)
	case strings.HasPrefix(text, "/search "):
		query := strings.TrimPrefix(text, "/search ")
		fb.searchFatwas(chatID, query, "keyword")
	case strings.HasPrefix(text, "/title "):
		query := strings.TrimPrefix(text, "/title ")
		fb.searchFatwas(chatID, query, "title")
	case strings.HasPrefix(text, "/category "):
		query := strings.TrimPrefix(text, "/category ")
		fb.searchFatwas(chatID, query, "category")
	case text == "/categories":
		fb.showCategories(chatID)
	default:
		// Default search by keyword
		fb.searchFatwas(chatID, text, "keyword")
	}
}

func (fb *FatwaBot) handleCallbackQuery(callbackQuery *tgbotapi.CallbackQuery) {
	chatID := callbackQuery.Message.Chat.ID
	data := callbackQuery.Data

	// Parse callback data (format: "view_ID")
	if strings.HasPrefix(data, "view_") {
		idStr := strings.TrimPrefix(data, "view_")
		id, err := strconv.Atoi(idStr)
		if err != nil {
			fb.sendMessage(chatID, "❌ Error parsing fatwa ID")
			return
		}

		// Find and display the fatwa
		for _, fatwa := range fb.fatwas {
			if fatwa.ID == id {
				fb.sendFatwaDetails(chatID, fatwa)
				break
			}
		}
	}

	// Answer callback query
	callback := tgbotapi.NewCallback(callbackQuery.ID, "")
	fb.bot.Request(callback)
}

func (fb *FatwaBot) sendWelcomeMessage(chatID int64) {
	message := `🕌 *Selamat Datang ke ApaHukumBot*

Bot ini membantu anda mencari fatwa daripada Jabatan Mufti Wilayah Persekutuan.

*Cara menggunakan:*
• Taip sebarang kata kunci untuk carian umum
• /search [kata kunci] - Cari dalam tajuk dan kandungan
• /title [kata kunci] - Cari berdasarkan tajuk sahaja  
• /category [kategori] - Cari berdasarkan kategori
• /categories - Lihat senarai kategori
• /help - Panduan lengkap

*Contoh:*
• "haiwan peliharaan"
• /title solat
• /category irsyad

Mulakan pencarian anda sekarang! 🔍

Created by @mnajmuddean
💬 Sebarang cadangan atau isu, sila hubungi: @mnajmuddean`

	msg := tgbotapi.NewMessage(chatID, message)
	msg.ParseMode = "Markdown"
	fb.bot.Send(msg)
}

func (fb *FatwaBot) sendHelpMessage(chatID int64) {
	message := "📚 *Panduan Penggunaan Bot Fatwa*\n\n" +
		"*Perintah Yang Tersedia:*\n\n" +
		"🔍 *Pencarian Umum*\n" +
		"• Taip sahaja kata kunci anda\n" +
		"• Contoh: \"zakat fitrah\"\n\n" +
		"🔍 *Pencarian Khusus*\n" +
		"• `/search [kata kunci]` - Cari dalam tajuk dan kandungan\n" +
		"• `/title [kata kunci]` - Cari berdasarkan tajuk sahaja\n" +
		"• `/category [kategori]` - Cari berdasarkan kategori\n\n" +
		"📂 *Kategori*\n" +
		"• `/categories` - Lihat semua kategori yang ada\n\n" +
		"ℹ️ *Maklumat Lain*\n" +
		"• `/help` - Papar panduan ini\n" +
		"• `/start` - Mula semula\n\n" +
		"*Tips Pencarian:*\n" +
		"• Gunakan kata kunci yang ringkas dan tepat\n" +
		"• Boleh guna Bahasa Malaysia atau Arab\n" +
		"• Cari menggunakan sebahagian tajuk untuk hasil yang lebih baik\n\n" +
		"Selamat mencari fatwa! 🤲"

	msg := tgbotapi.NewMessage(chatID, message)
	msg.ParseMode = "Markdown"
	fb.bot.Send(msg)
}

func (fb *FatwaBot) searchFatwas(chatID int64, query string, searchType string) {
	if strings.TrimSpace(query) == "" {
		fb.sendMessage(chatID, "❌ Sila masukkan kata kunci untuk carian")
		return
	}

	fb.sendMessage(chatID, "🔍 Mencari fatwa...")

	var results []Fatwa
	query = strings.ToLower(query)

	for _, fatwa := range fb.fatwas {
		var match bool

		switch searchType {
		case "title":
			match = strings.Contains(strings.ToLower(fatwa.Title), query)
		case "category":
			match = strings.Contains(strings.ToLower(fatwa.Category), query)
		case "keyword":
			match = strings.Contains(strings.ToLower(fatwa.Title), query) ||
				strings.Contains(strings.ToLower(fatwa.Content), query)
		}

		if match {
			results = append(results, fatwa)
		}
	}

	if len(results) == 0 {
		fb.sendMessage(chatID, fmt.Sprintf("❌ Tiada fatwa dijumpai untuk: *%s*", query))
		return
	}

	// Limit results to avoid message being too long
	maxResults := 10
	if len(results) > maxResults {
		results = results[:maxResults]
	}

	fb.sendSearchResults(chatID, results, query, len(results) < len(fb.fatwas))
}

func (fb *FatwaBot) sendSearchResults(chatID int64, results []Fatwa, query string, isLimited bool) {
	message := fmt.Sprintf("🔍 *Hasil carian untuk: %s*\n\n", query)

	if isLimited && len(results) >= 10 {
		message += "📝 *Paparan 10 hasil pertama*\n\n"
	}

	// Create inline keyboard
	var keyboard [][]tgbotapi.InlineKeyboardButton

	for i, fatwa := range results {
		// Add result text
		message += fmt.Sprintf("*%d. %s*\n", i+1, fatwa.Title)
		message += fmt.Sprintf("📅 %s | 👁 %d views\n", fatwa.Date, fatwa.Hits)

		// Show preview of content (first 100 characters)
		preview := fatwa.Content
		if len(preview) > 100 {
			preview = preview[:100] + "..."
		}
		message += fmt.Sprintf("📄 %s\n\n", preview)

		// Add inline button for this fatwa
		button := tgbotapi.NewInlineKeyboardButtonData(
			fmt.Sprintf("📖 Baca Fatwa %d", i+1),
			fmt.Sprintf("view_%d", fatwa.ID),
		)
		keyboard = append(keyboard, []tgbotapi.InlineKeyboardButton{button})
	}

	msg := tgbotapi.NewMessage(chatID, message)
	msg.ParseMode = "Markdown"

	if len(keyboard) > 0 {
		msg.ReplyMarkup = tgbotapi.NewInlineKeyboardMarkup(keyboard...)
	}

	fb.bot.Send(msg)
}

func (fb *FatwaBot) sendFatwaDetails(chatID int64, fatwa Fatwa) {
	// Split content into chunks if it's too long
	const maxMessageLength = 4096

	header := fmt.Sprintf("📖 *%s*\n\n", fatwa.Title)
	header += fmt.Sprintf("🆔 ID: %d\n", fatwa.ID)
	header += fmt.Sprintf("📅 Tarikh: %s\n", fatwa.Date)
	header += fmt.Sprintf("👁 Paparan: %d\n", fatwa.Hits)
	header += fmt.Sprintf("📂 Kategori: %s\n\n", fatwa.Category)

	content := fatwa.Content
	footer := fmt.Sprintf("\n\n🔗 [Baca penuh di laman web](%s)", fatwa.URL)

	// Check if we need to split the message
	fullMessage := header + content + footer

	if len(fullMessage) <= maxMessageLength {
		// Send as single message
		msg := tgbotapi.NewMessage(chatID, fullMessage)
		msg.ParseMode = "Markdown"
		msg.DisableWebPagePreview = true
		fb.bot.Send(msg)
	} else {
		// Send header first
		msg := tgbotapi.NewMessage(chatID, header)
		msg.ParseMode = "Markdown"
		fb.bot.Send(msg)

		// Split content into chunks
		contentChunks := fb.splitText(content, maxMessageLength-200) // Leave space for formatting

		for i, chunk := range contentChunks {
			chunkMsg := fmt.Sprintf("📄 *Bahagian %d/%d*\n\n%s", i+1, len(contentChunks), chunk)
			msg := tgbotapi.NewMessage(chatID, chunkMsg)
			msg.ParseMode = "Markdown"
			fb.bot.Send(msg)
		}

		// Send footer with link
		msg = tgbotapi.NewMessage(chatID, footer)
		msg.ParseMode = "Markdown"
		msg.DisableWebPagePreview = true
		fb.bot.Send(msg)
	}
}

func (fb *FatwaBot) splitText(text string, maxLength int) []string {
	if len(text) <= maxLength {
		return []string{text}
	}

	var chunks []string
	sentences := strings.Split(text, ".")

	currentChunk := ""
	for _, sentence := range sentences {
		if len(currentChunk)+len(sentence)+1 <= maxLength {
			if currentChunk != "" {
				currentChunk += "."
			}
			currentChunk += sentence
		} else {
			if currentChunk != "" {
				chunks = append(chunks, currentChunk)
			}
			currentChunk = sentence
		}
	}

	if currentChunk != "" {
		chunks = append(chunks, currentChunk)
	}

	return chunks
}

func (fb *FatwaBot) showCategories(chatID int64) {
	categories := make(map[string]int)

	for _, fatwa := range fb.fatwas {
		categories[fatwa.Category]++
	}

	message := "📂 *Kategori Fatwa Yang Tersedia:*\n\n"

	for category, count := range categories {
		message += fmt.Sprintf("• %s (%d)\n", category, count)
	}

	message += "\n💡 *Cara mencari berdasarkan kategori:*\n"
	message += "`/category [nama kategori]`\n\n"
	message += "*Contoh:* `/category irsyad`"

	msg := tgbotapi.NewMessage(chatID, message)
	msg.ParseMode = "Markdown"
	fb.bot.Send(msg)
}

func (fb *FatwaBot) sendMessage(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	msg.ParseMode = "Markdown"
	fb.bot.Send(msg)
}

func loadFatwaData(filename string) ([]Fatwa, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("cannot open CSV file: %v", err)
	}
	defer file.Close()

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("cannot read CSV file: %v", err)
	}

	if len(records) < 2 {
		return nil, fmt.Errorf("CSV file must have at least header and one data row")
	}

	var fatwas []Fatwa

	// Skip header row
	for i := 1; i < len(records); i++ {
		record := records[i]
		if len(record) < 7 {
			continue // Skip invalid records
		}

		id, _ := strconv.Atoi(record[0])
		hits, _ := strconv.Atoi(record[4])

		fatwa := Fatwa{
			ID:       id,
			Title:    record[1],
			URL:      record[2],
			Date:     record[3],
			Hits:     hits,
			Category: record[5],
			Content:  record[6],
		}

		fatwas = append(fatwas, fatwa)
	}

	return fatwas, nil
}

func isLastDayOfMonth() bool {
	now := time.Now()
	tomorrow := now.AddDate(0, 0, 1)
	return now.Month() != tomorrow.Month()
}

// Option 1: Single page scraping with content extraction
func singlePageScraping() {
	// Get the token
	muftiwpURL := os.Getenv("MUFTIWP_URL")
	if muftiwpURL == "" {
		log.Fatal("MUFTIWP_URL not set in environment")
	}

	baseURL := muftiwpURL + "ms/artikel/irsyad-hukum/umum?filter-search=&limit=0&filter_order=&filter_order_Dir=&limitstart=&task=&filter_submit="

	articles, err := scrapeArticles(baseURL)
	if err != nil {
		log.Fatalf("Error scraping articles: %v", err)
	}

	if len(articles) == 0 {
		log.Println("No articles found")
		return
	}

	// Extract content for each article
	fmt.Println("Extracting content from each article...")
	for i := range articles {
		content, err := extractArticleContent(articles[i].URL)
		if err != nil {
			fmt.Printf("Error extracting content from %s: %v\n", articles[i].URL, err)
			articles[i].Content = "Error extracting content"
		} else {
			articles[i].Content = content
		}
		fmt.Printf("Processed article %d/%d: %s\n", i+1, len(articles), articles[i].Title)

		// Add a small delay to be respectful to the server
		time.Sleep(1 * time.Second)
	}

	err = exportToCSV(articles, "fatwa.csv")
	if err != nil {
		log.Fatalf("Error exporting to CSV: %v", err)
	}

	fmt.Printf("Successfully scraped %d articles with content and exported to fatwa.csv\n", len(articles))
}

func scrapeArticles(url string) ([]Fatwa, error) {
	fmt.Printf("Scraping page: %s\n", url)

	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 10000 * time.Second,
	}

	// Make HTTP request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request: %v", err)
	}

	// Set headers to mimic a real browser
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Connection", "keep-alive")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error making request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("status code error: %d %s", resp.StatusCode, resp.Status)
	}

	// Handle gzip compression
	var reader io.Reader = resp.Body
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gzipReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("error creating gzip reader: %v", err)
		}
		defer gzipReader.Close()
		reader = gzipReader
	}

	// Parse HTML document
	doc, err := goquery.NewDocumentFromReader(reader)
	if err != nil {
		return nil, fmt.Errorf("error parsing HTML: %v", err)
	}

	var articles []Fatwa

	// Debug: Print the HTML structure to understand the page layout
	fmt.Printf("Page title: %s\n", doc.Find("title").Text())

	// Try multiple selectors to find the articles
	selectors := []string{
		"table.category tbody tr",
		".category tbody tr",
		"tbody tr",
		".list-item",
		".article-item",
		"tr",
	}

	var foundArticles bool
	for _, selector := range selectors {
		doc.Find(selector).Each(func(i int, s *goquery.Selection) {
			article := Fatwa{}

			// Try different selectors for title and URL
			var titleElement *goquery.Selection
			titleSelectors := []string{
				"td.list-title a",
				".list-title a",
				"td a",
				"a[href*='artikel']",
				"a",
			}

			for _, titleSel := range titleSelectors {
				titleElement = s.Find(titleSel)
				if titleElement.Length() > 0 {
					break
				}
			}

			if titleElement != nil && titleElement.Length() > 0 {
				article.Title = strings.TrimSpace(titleElement.Text())
				href, exists := titleElement.Attr("href")
				if exists {
					// Convert relative URL to absolute URL
					if strings.HasPrefix(href, "/") {
						article.URL = "https://www.muftiwp.gov.my" + href
					} else {
						article.URL = href
					}
				}
			}

			// Try different selectors for date
			dateSelectors := []string{
				"td.list-date",
				".list-date",
				"td:nth-child(3)",
				".date",
			}

			for _, dateSel := range dateSelectors {
				dateCell := s.Find(dateSel)
				if dateCell.Length() > 0 {
					article.Date = strings.TrimSpace(dateCell.Text())
					break
				}
			}

			// Try different selectors for hits
			hitsSelectors := []string{
				"td.list-hits span.badge",
				".list-hits .badge",
				"td:nth-child(4) span",
				".hits",
				"span.badge",
			}

			for _, hitsSel := range hitsSelectors {
				hitsCell := s.Find(hitsSel)
				if hitsCell.Length() > 0 {
					hitsText := strings.TrimSpace(hitsCell.Text())
					// Extract number from "Dikunjungi: 31" format
					re := regexp.MustCompile(`(?:Dikunjungi:\s*)?(\d+)`)
					matches := re.FindStringSubmatch(hitsText)
					if len(matches) > 1 {
						hits, err := strconv.Atoi(matches[1])
						if err == nil {
							article.Hits = hits
						}
					}
					break
				}
			}

			// Extract article ID from URL if possible
			if article.URL != "" {
				re := regexp.MustCompile(`/(\d+)-`)
				matches := re.FindStringSubmatch(article.URL)
				if len(matches) > 1 {
					id, err := strconv.Atoi(matches[1])
					if err == nil {
						article.ID = id
					}
				}
			}

			// Set category
			article.Category = "Irsyad Hukum - Umum"

			// Only add if we have essential data
			if article.Title != "" && article.URL != "" {
				articles = append(articles, article)
				foundArticles = true
			}
		})

		if foundArticles {
			break
		}
	}

	if !foundArticles {
		// Debug: Print page content to help identify the structure
		fmt.Println("No articles found with any selector. Page content preview:")
		fmt.Println(doc.Find("body").Text()[:min(500, len(doc.Find("body").Text()))])
	}

	return articles, nil
}

// New function to extract article content from individual article pages
func extractArticleContent(url string) (string, error) {
	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 30 * time.Second,
	}

	// Make HTTP request
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", fmt.Errorf("error creating request: %v", err)
	}

	// Set headers to mimic a real browser
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	req.Header.Set("Connection", "keep-alive")

	resp, err := client.Do(req)
	if err != nil {
		return "", fmt.Errorf("error making request: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", fmt.Errorf("status code error: %d %s", resp.StatusCode, resp.Status)
	}

	// Handle gzip compression
	var reader io.Reader = resp.Body
	if resp.Header.Get("Content-Encoding") == "gzip" {
		gzipReader, err := gzip.NewReader(resp.Body)
		if err != nil {
			return "", fmt.Errorf("error creating gzip reader: %v", err)
		}
		defer gzipReader.Close()
		reader = gzipReader
	}

	// Parse HTML document
	doc, err := goquery.NewDocumentFromReader(reader)
	if err != nil {
		return "", fmt.Errorf("error parsing HTML: %v", err)
	}

	// Extract content from div with itemprop="articleBody"
	articleBody := doc.Find("div[itemprop='articleBody']")
	if articleBody.Length() == 0 {
		// Try alternative selectors if the primary one doesn't work
		alternativeSelectors := []string{
			".article-body",
			".content",
			"#article-content",
			".post-content",
		}

		for _, selector := range alternativeSelectors {
			articleBody = doc.Find(selector)
			if articleBody.Length() > 0 {
				break
			}
		}
	}

	if articleBody.Length() == 0 {
		return "", fmt.Errorf("article body not found")
	}

	// Extract text content and clean it up
	content := articleBody.Text()

	// Clean up the content
	content = strings.TrimSpace(content)

	// Replace multiple whitespaces with single space
	re := regexp.MustCompile(`\s+`)
	content = re.ReplaceAllString(content, " ")

	// Remove excessive newlines
	content = strings.ReplaceAll(content, "\n\n\n", "\n\n")

	return content, nil
}

func exportToCSV(articles []Fatwa, filename string) error {
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("cannot create CSV file: %v", err)
	}
	defer file.Close()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// Write CSV header - now includes Content column
	header := []string{"ID", "Title", "URL", "Date", "Hits", "Category", "Content"}
	if err := writer.Write(header); err != nil {
		return fmt.Errorf("error writing CSV header: %v", err)
	}

	// Write article data
	for _, article := range articles {
		record := []string{
			strconv.Itoa(article.ID),
			article.Title,
			article.URL,
			article.Date,
			strconv.Itoa(article.Hits),
			article.Category,
			article.Content, // New content field
		}
		if err := writer.Write(record); err != nil {
			return fmt.Errorf("error writing CSV record: %v", err)
		}
	}

	fmt.Printf("CSV file '%s' created successfully with %d records\n", filename, len(articles))
	return nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
