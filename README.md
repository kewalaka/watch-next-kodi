# "What's Next" - A Kodi Watchlist Manager

## Tech Stack
- **Backend:** Go (Standard Library + SQLite)
- **Frontend:** React + Vite + Tailwind + TanStack Query
- **Proxy:** Integrated static file server in Go

## Local Development

### Backend
```bash
go run main.go
# Env: MOCK_KODI=true (optional)
```

### Frontend
```bash
cd web
npm install
npm run dev
```

## Production Build (Mac to x64)

Built for local transfer without a registry.

```bash
# Build for target architecture
docker build --platform linux/amd64 -t whats-next .

# Package for transfer
docker save whats-next | gzip > whats-next.tar.gz
```

## Deployment

1. Transfer `whats-next.tar.gz` and `docker-compose.yml` to the server.
2. Load and run:

```bash
gunzip -c whats-next.tar.gz | docker load
docker-compose up -d
```

## Persistence
- `whats-next.db`: SQLite database (Lists, Watchlist, Kodi Library Cache).
- `./data/posters/`: Local cache of portrait posters (Boxset look). Deduplicated by `Title + Year`.

## Possibility of linkage via intent

<https://community.yatse.tv/t/intent-to-land-on-a-specific-movie-or-tv-show-based-on-kodi-id/5235>

Current intent API:

<https://yatse.tv/wiki/yatse-api>