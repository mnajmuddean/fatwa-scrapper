# README.md

# Fatwa Scrapper & Telegram Bot

This application scrapes fatwa articles from the Jabatan Mufti Wilayah Persekutuan website and provides a Telegram bot interface for searching and retrieving fatwas. Users can search by keyword, title, or category, and receive results directly in Telegram.

## Features

- Scheduled scraping of fatwa articles (monthly, via cron)
- Stores fatwa data in a CSV file
- Telegram bot for searching fatwas by keyword, title, or category
- Category listing and detailed fatwa view
- Written in Go

## Tech Stack

- **Language:** Go
- **Libraries:**
  - [goquery](https://github.com/PuerkitoBio/goquery) (HTML scraping)
  - [telegram-bot-api](https://github.com/go-telegram-bot-api/telegram-bot-api) (Telegram bot)
  - [robfig/cron](https://github.com/robfig/cron) (Scheduling)
  - [godotenv](https://github.com/joho/godotenv) (Environment variables)

## Prerequisites

- Go 1.24+
- Telegram account (to create a bot and get a token)

## Setup

1. **Clone the repository:**

   ```sh
   git clone https://github.com/yourusername/fatwa-scrapper.git
   cd fatwa-scrapper
   ```

2. **Install dependencies:**

   ```sh
   go mod tidy
   ```

3. **Configure environment variables:**

   - Copy `.env.example` to `.env` and fill in your Telegram bot token.

4. **Build the app:**

   ```sh
   go build -o fatwa-scrapper
   ```

5. **Run the app:**

   ```sh
   ./fatwa-scrapper
   ```

   The bot will start and scraping will be scheduled automatically.

## Deployment

- Deploy as a long-running process on your server (e.g., using `systemd`, `pm2`, or Docker).
- Ensure your `.env` file is present and contains the correct `BOT_TOKEN`.
- The app will handle scraping and bot operations automatically.

## License

MIT
