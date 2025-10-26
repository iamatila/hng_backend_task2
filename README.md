# Country Currency & Exchange API

A RESTful API built with Go and Fiber that fetches country data, exchange rates, and provides CRUD operations with image generation capabilities.

## Prerequisites

- Go 1.21 or higher
- MySQL 5.7 or higher
- Git

## Installation

### Local Development

#### 1. Clone the Repository

```bash
git clone <repository-url>
cd hnd_backend_task2
```

#### 2. Install Dependencies

```bash
go mod download
```

#### 3. Setup MySQL Database

```sql
CREATE DATABASE countries_db CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci;
```

#### 4. Configure Environment

Copy `.env.example` to `.env` and update with your database credentials:

```bash
cp .env.example .env
```

Edit `.env`:
```
PORT=3000
DB_HOST=localhost
DB_PORT=3306
DB_USER=root
DB_PASSWORD=your_password
DB_NAME=countries_db
```

#### 5. Run the Application

```bash
go run main.go
```

The server will start on `http://localhost:3000`

---

## API Endpoints

### 1. Refresh Countries Data

**POST** `/countries/refresh`

Fetches all countries and exchange rates from external APIs and stores them in the database.

**Response:**
```json
{
  "message": "Countries refreshed successfully",
  "total_processed": 250,
  "last_refreshed_at": "2025-10-22T18:00:00Z"
}
```

**Error Response (503):**
```json
{
  "error": "External data source unavailable",
  "details": "Could not fetch data from restcountries API: ..."
}
```

### 2. Get All Countries

**GET** `/countries`

Retrieve all countries with optional filters and sorting.

**Query Parameters:**
- `region` - Filter by region (e.g., `Africa`, `Europe`)
- `currency` - Filter by currency code (e.g., `NGN`, `USD`)
- `sort` - Sort results:
  - `gdp_desc` - Highest GDP first
  - `gdp_asc` - Lowest GDP first
  - `population_desc` - Highest population first
  - `population_asc` - Lowest population first

**Examples:**

```bash
# Get all African countries
GET /countries?region=Africa

# Get countries using NGN currency
GET /countries?currency=NGN

# Get countries sorted by GDP (descending)
GET /countries?sort=gdp_desc

# Combine filters
GET /countries?region=Africa&sort=gdp_desc
```

**Response:**
```json
[
  {
    "id": 1,
    "name": "Nigeria",
    "capital": "Abuja",
    "region": "Africa",
    "population": 206139589,
    "currency_code": "NGN",
    "exchange_rate": 1600.23,
    "estimated_gdp": 25767448125.2,
    "flag_url": "https://flagcdn.com/ng.svg",
    "last_refreshed_at": "2025-10-22T18:00:00Z"
  }
]
```

### 3. Get Single Country

**GET** `/countries/:name`

Get a specific country by name (case-insensitive).

**Example:**
```bash
GET /countries/nigeria
```

**Response:**
```json
{
  "id": 1,
  "name": "Nigeria",
  "capital": "Abuja",
  "region": "Africa",
  "population": 206139589,
  "currency_code": "NGN",
  "exchange_rate": 1600.23,
  "estimated_gdp": 25767448125.2,
  "flag_url": "https://flagcdn.com/ng.svg",
  "last_refreshed_at": "2025-10-22T18:00:00Z"
}
```

**Error Response (404):**
```json
{
  "error": "Country not found"
}
```

### 4. Delete Country

**DELETE** `/countries/:name`

Delete a country record by name (case-insensitive).

**Example:**
```bash
DELETE /countries/nigeria
```

**Response:**
```json
{
  "message": "Country deleted successfully"
}
```

### 5. Get Status

**GET** `/status`

Get total countries count and last refresh timestamp.

**Response:**
```json
{
  "total_countries": 250,
  "last_refreshed_at": "2025-10-22T18:00:00Z"
}
```

### 6. Get Summary Image

**GET** `/countries/image`

Retrieve the auto-generated summary image showing:
- Total number of countries
- Top 5 countries by estimated GDP
- Last refresh timestamp

**Response:** PNG image file

**Error Response (404):**
```json
{
  "error": "Summary image not found"
}
```

## Data Processing Logic

### Currency Handling

1. **Multiple Currencies**: Only the first currency code is stored
2. **Empty Currencies**: 
   - `currency_code` → `null`
   - `exchange_rate` → `null`
   - `estimated_gdp` → `0`
3. **Missing Exchange Rate**: 
   - `exchange_rate` → `null`
   - `estimated_gdp` → `null`

### GDP Calculation

```
estimated_gdp = population × random(1000-2000) ÷ exchange_rate
```

- Random multiplier regenerated on each refresh
- Provides unique GDP estimates per refresh cycle

### Update Logic

- Matches existing countries by name (case-insensitive)
- Updates all fields including recalculated GDP
- Inserts new records if country doesn't exist

## Project Structure

```
hnd_backend_task2/
├── main.go           # Main application file
├── go.mod            # Go module dependencies
├── go.sum            # Dependency checksums
├── .env              # Environment configuration
├── .env.example      # Example environment file
├── README.md         # This file
└── cache/            # Generated images directory
    └── summary.png   # Auto-generated summary image
```

## External APIs

- **Countries Data**: https://restcountries.com/v2/all
- **Exchange Rates**: https://open.er-api.com/v6/latest/USD

## Error Handling

All errors return consistent JSON responses:

- `400` - Validation failed
- `404` - Resource not found
- `500` - Internal server error
- `503` - External service unavailable

## Development

### Building for Production

```bash
go build -o country-api main.go
./country-api
```

### Running Tests

```bash
go test ./...
```

## Testing with cURL

```bash
# Refresh data
curl -X POST http://localhost:3000/countries/refresh

# Get all countries
curl http://localhost:3000/countries

# Filter by region
curl http://localhost:3000/countries?region=Africa

# Sort by GDP
curl http://localhost:3000/countries?sort=gdp_desc

# Get specific country
curl http://localhost:3000/countries/Nigeria

# Get status
curl http://localhost:3000/status

# Download summary image
curl http://localhost:3000/countries/image --output summary.png

# Delete a country
curl -X DELETE http://localhost:3000/countries/Nigeria
```

## Technologies Used

- **Framework**: [Fiber](https://gofiber.io/) - Fast HTTP web framework
- **ORM**: [GORM](https://gorm.io/) - Go ORM library
- **Database**: MySQL 5.7+
- **Image Processing**: golang.org/x/image
- **Environment**: godotenv
