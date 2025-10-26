package main

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/joho/godotenv"
	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

// Country model
type Country struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	Name            string    `gorm:"type:varchar(255);uniqueIndex;not null" json:"name"`
	Capital         *string   `gorm:"type:varchar(255)" json:"capital"`
	Region          *string   `gorm:"type:varchar(100)" json:"region"`
	Population      int64     `gorm:"not null" json:"population"`
	CurrencyCode    *string   `gorm:"type:varchar(10)" json:"currency_code"`
	ExchangeRate    *float64  `json:"exchange_rate"`
	EstimatedGDP    *float64  `json:"estimated_gdp"`
	FlagURL         *string   `gorm:"type:varchar(500)" json:"flag_url"`
	LastRefreshedAt time.Time `json:"last_refreshed_at"`
}

// External API response structures
type RestCountry struct {
	Name       string              `json:"name"`
	Capital    string              `json:"capital"`
	Region     string              `json:"region"`
	Population int64               `json:"population"`
	Flag       string              `json:"flag"`
	Currencies []map[string]string `json:"currencies"`
}

type ExchangeRateResponse struct {
	Rates map[string]float64 `json:"rates"`
}

var db *gorm.DB

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	// Connect to database
	initDB()

	// Create cache directory
	os.MkdirAll("cache", os.ModePerm)

	// Initialize Fiber app
	app := fiber.New(fiber.Config{
		ErrorHandler: customErrorHandler,
	})

	// Middleware
	app.Use(logger.New())
	app.Use(cors.New())

	// Routes
	app.Post("/countries/refresh", refreshCountries)
	app.Get("/countries", getCountries)
	app.Get("/countries/image", getCountriesImage)
	app.Get("/countries/:name", getCountryByName)
	app.Delete("/countries/:name", deleteCountry)
	app.Get("/status", getStatus)

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "3000"
	}

	log.Printf("Server starting on port %s", port)
	log.Fatal(app.Listen(":" + port))
}

func initDB() {
	var dsn string

	// Check if DATABASE_URL exists (Railway, Heroku, etc.)
	if databaseURL := os.Getenv("DATABASE_URL"); databaseURL != "" {
		// Railway and other platforms provide DATABASE_URL
		// Convert postgres:// to mysql:// if needed, or use as-is for MySQL
		dsn = convertDatabaseURL(databaseURL)
		log.Println("Using DATABASE_URL from environment")
	} else {
		// Local development - use individual env variables
		dsn = fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			getEnv("DB_USER", "root"),
			getEnv("DB_PASSWORD", ""),
			getEnv("DB_HOST", "localhost"),
			getEnv("DB_PORT", "3306"),
			getEnv("DB_NAME", "countries_db"),
		)
		log.Println("Using individual database env variables")
	}

	var err error
	db, err = gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Printf("Failed to connect to database: %v", err)
		log.Printf("DSN used (password hidden): %s", hideSensitiveInfo(dsn))
		log.Fatal("Database connection failed")
	}

	// Auto migrate
	if err := db.AutoMigrate(&Country{}); err != nil {
		log.Fatal("Failed to migrate database:", err)
	}

	log.Println("Database connected successfully")
}

func convertDatabaseURL(databaseURL string) string {
	// Railway MySQL format: mysql://user:password@host:port/database
	// GORM MySQL DSN format: user:password@tcp(host:port)/database?params

	// If it already starts with mysql://, convert it
	if len(databaseURL) > 8 && databaseURL[:8] == "mysql://" {
		// Remove mysql:// prefix
		databaseURL = databaseURL[8:]

		// Split user:pass and host:port/db
		parts := strings.Split(databaseURL, "@")
		if len(parts) != 2 {
			return databaseURL
		}

		userPass := parts[0]
		hostPortDb := parts[1]

		// Check if host:port/db format
		dbParts := strings.Split(hostPortDb, "/")
		if len(dbParts) != 2 {
			return databaseURL
		}

		hostPort := dbParts[0]
		dbName := dbParts[1]

		// Construct GORM-compatible DSN
		return fmt.Sprintf("%s@tcp(%s)/%s?charset=utf8mb4&parseTime=True&loc=Local", userPass, hostPort, dbName)
	}

	return databaseURL
}

func hideSensitiveInfo(dsn string) string {
	// Hide password in logs
	parts := strings.Split(dsn, "@")
	if len(parts) > 1 {
		userPass := strings.Split(parts[0], ":")
		if len(userPass) > 1 {
			return userPass[0] + ":****@" + parts[1]
		}
	}
	return dsn
}

func refreshCountries(c *fiber.Ctx) error {
	// Fetch countries
	countries, err := fetchCountries()
	if err != nil {
		return c.Status(503).JSON(fiber.Map{
			"error":   "External data source unavailable",
			"details": fmt.Sprintf("Could not fetch data from restcountries API: %v", err),
		})
	}

	// Fetch exchange rates
	rates, err := fetchExchangeRates()
	if err != nil {
		return c.Status(503).JSON(fiber.Map{
			"error":   "External data source unavailable",
			"details": fmt.Sprintf("Could not fetch data from exchange rates API: %v", err),
		})
	}

	now := time.Now()
	rand.Seed(time.Now().UnixNano())

	// Process and save countries
	for _, country := range countries {
		var currencyCode *string
		var exchangeRate *float64
		var estimatedGDP *float64

		// Handle currency
		if len(country.Currencies) > 0 && country.Currencies[0]["code"] != "" {
			code := country.Currencies[0]["code"]
			currencyCode = &code

			// Get exchange rate
			if rate, exists := rates[code]; exists {
				exchangeRate = &rate

				// Calculate estimated GDP
				randomMultiplier := rand.Float64()*(2000-1000) + 1000
				gdp := float64(country.Population) * randomMultiplier / rate
				estimatedGDP = &gdp
			}
		} else {
			// Empty currencies array
			gdp := 0.0
			estimatedGDP = &gdp
		}

		capital := country.Capital
		region := country.Region
		flagURL := country.Flag

		dbCountry := Country{
			Name:            country.Name,
			Capital:         nilIfEmpty(&capital),
			Region:          nilIfEmpty(&region),
			Population:      country.Population,
			CurrencyCode:    currencyCode,
			ExchangeRate:    exchangeRate,
			EstimatedGDP:    estimatedGDP,
			FlagURL:         nilIfEmpty(&flagURL),
			LastRefreshedAt: now,
		}

		// Upsert (update or insert)
		var existing Country
		result := db.Where("LOWER(name) = LOWER(?)", country.Name).First(&existing)

		if result.Error == gorm.ErrRecordNotFound {
			// Insert new
			db.Create(&dbCountry)
		} else {
			// Update existing
			db.Model(&existing).Updates(dbCountry)
		}
	}

	// Generate summary image
	if err := generateSummaryImage(); err != nil {
		log.Printf("Failed to generate summary image: %v", err)
	}

	return c.JSON(fiber.Map{
		"message":           "Countries refreshed successfully",
		"total_processed":   len(countries),
		"last_refreshed_at": now,
	})
}

func getCountries(c *fiber.Ctx) error {
	var countries []Country
	query := db.Model(&Country{})

	// Filters
	if region := c.Query("region"); region != "" {
		query = query.Where("region = ?", region)
	}

	if currency := c.Query("currency"); currency != "" {
		query = query.Where("currency_code = ?", currency)
	}

	// Sorting
	sortBy := c.Query("sort")
	switch sortBy {
	case "gdp_desc":
		query = query.Order("estimated_gdp DESC")
	case "gdp_asc":
		query = query.Order("estimated_gdp ASC")
	case "population_desc":
		query = query.Order("population DESC")
	case "population_asc":
		query = query.Order("population ASC")
	default:
		query = query.Order("name ASC")
	}

	if err := query.Find(&countries).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "Internal server error",
		})
	}

	return c.JSON(countries)
}

func getCountryByName(c *fiber.Ctx) error {
	name := c.Params("name")
	var country Country

	if err := db.Where("LOWER(name) = LOWER(?)", name).First(&country).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return c.Status(404).JSON(fiber.Map{
				"error": "Country not found",
			})
		}
		return c.Status(500).JSON(fiber.Map{
			"error": "Internal server error",
		})
	}

	return c.JSON(country)
}

func deleteCountry(c *fiber.Ctx) error {
	name := c.Params("name")
	var country Country

	result := db.Where("LOWER(name) = LOWER(?)", name).First(&country)
	if result.Error == gorm.ErrRecordNotFound {
		return c.Status(404).JSON(fiber.Map{
			"error": "Country not found",
		})
	}

	if err := db.Delete(&country).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{
			"error": "Internal server error",
		})
	}

	return c.JSON(fiber.Map{
		"message": "Country deleted successfully",
	})
}

func getStatus(c *fiber.Ctx) error {
	var count int64
	db.Model(&Country{}).Count(&count)

	var lastRefresh time.Time
	db.Model(&Country{}).Select("MAX(last_refreshed_at)").Scan(&lastRefresh)

	return c.JSON(fiber.Map{
		"total_countries":   count,
		"last_refreshed_at": lastRefresh,
	})
}

func getCountriesImage(c *fiber.Ctx) error {
	imagePath := "cache/summary.png"
	if _, err := os.Stat(imagePath); os.IsNotExist(err) {
		return c.Status(404).JSON(fiber.Map{
			"error": "Summary image not found",
		})
	}

	return c.SendFile(imagePath)
}

// Helper functions
func fetchCountries() ([]RestCountry, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get("https://restcountries.com/v2/all?fields=name,capital,region,population,flag,currencies")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var countries []RestCountry
	if err := json.Unmarshal(body, &countries); err != nil {
		return nil, err
	}

	return countries, nil
}

func fetchExchangeRates() (map[string]float64, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get("https://open.er-api.com/v6/latest/USD")
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("API returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var ratesResp ExchangeRateResponse
	if err := json.Unmarshal(body, &ratesResp); err != nil {
		return nil, err
	}

	return ratesResp.Rates, nil
}

func generateSummaryImage() error {
	// Get total countries
	var totalCount int64
	db.Model(&Country{}).Count(&totalCount)

	// Get top 5 by GDP
	var topCountries []Country
	db.Order("estimated_gdp DESC").Limit(5).Find(&topCountries)

	// Get last refresh time
	var lastRefresh time.Time
	db.Model(&Country{}).Select("MAX(last_refreshed_at)").Scan(&lastRefresh)

	// Create image
	img := image.NewRGBA(image.Rect(0, 0, 600, 400))
	bgColor := color.RGBA{240, 240, 250, 255}
	draw.Draw(img, img.Bounds(), &image.Uniform{bgColor}, image.Point{}, draw.Src)

	// Draw text
	col := color.RGBA{0, 0, 0, 255}
	point := fixed.Point26_6{X: fixed.I(20), Y: fixed.I(30)}

	// Title
	addLabel(img, point, "Country Currency & Exchange Summary", col)
	point.Y += fixed.I(40)

	// Total countries
	addLabel(img, point, fmt.Sprintf("Total Countries: %d", totalCount), col)
	point.Y += fixed.I(30)

	// Top 5
	addLabel(img, point, "Top 5 Countries by Estimated GDP:", col)
	point.Y += fixed.I(25)

	for i, country := range topCountries {
		gdpStr := "N/A"
		if country.EstimatedGDP != nil {
			gdpStr = fmt.Sprintf("$%.2f", *country.EstimatedGDP)
		}
		text := fmt.Sprintf("%d. %s - %s", i+1, country.Name, gdpStr)
		addLabel(img, point, text, col)
		point.Y += fixed.I(25)
	}

	point.Y += fixed.I(20)
	addLabel(img, point, fmt.Sprintf("Last Refreshed: %s", lastRefresh.Format(time.RFC3339)), col)

	// Save image
	file, err := os.Create("cache/summary.png")
	if err != nil {
		return err
	}
	defer file.Close()

	return png.Encode(file, img)
}

func addLabel(img *image.RGBA, point fixed.Point26_6, label string, col color.Color) {
	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(col),
		Face: basicfont.Face7x13,
		Dot:  point,
	}
	d.DrawString(label)
}

func nilIfEmpty(s *string) *string {
	if s == nil || *s == "" {
		return nil
	}
	return s
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func customErrorHandler(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	message := "Internal server error"

	if e, ok := err.(*fiber.Error); ok {
		code = e.Code
		message = e.Message
	}

	return c.Status(code).JSON(fiber.Map{
		"error": message,
	})
}
