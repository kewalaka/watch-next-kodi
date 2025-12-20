# "What's Next" - A Kodi Watchlist Manager

A sleek, lightweight web dashboard for managing multiple Kodi watchlists across different rooms.

## Tech Stack
- **Backend:** Go 1.25 (Standard Library + SQLite)
- **Frontend:** React + Vite + Tailwind + TanStack Query
- **CI/CD:** GitHub Actions with Trivy Security Scanning
- **Containerization:** Docker (linux/amd64)

## Configuration

Create a `config.json` in the root directory (see `config.example.json` for structure):

```json
{
    "subtitle": "A watchlist manager for Kodi",
    "footer": "Made with Antigravity by kewalaka",
    "lists": [
        {
            "group_name": "Bedroom",
            "type": "movies",
            "kodi_host": "https://kodi1",
            "username": "kodi",
            "password": "password"
        }
    ]
}
```

## Local Development

### Backend
```bash
# Serves API on :8090
go run main.go
# Env: MOCK_KODI=true (if you don't have a Kodi instance reachable)
```

### Frontend
```bash
cd web
npm install
npm run dev # Serves UI on :5173 with proxy to :8090
```

## Deployment

### via GitHub Packages (GHCR)
The project is set up with GitHub Actions:
- **Push to `main`**: Builds and pushes `ghcr.io/kewalaka/watch-next-kodi:latest`.
- **Push a tag (`v*`)**: Promotes the latest build to a versioned tag.

### Manual Docker Build
```bash
docker build -t watch-next-kodi .
docker-compose up -d
```

## Persistence
All persistent data is stored in the `./data` directory:
- `data/whats-next.db`: SQLite database.
- `data/posters/`: Local cache of portrait posters.

## License
MIT License - Copyright (c) 2025 kewalaka