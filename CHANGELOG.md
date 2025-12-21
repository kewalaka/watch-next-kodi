# Changelog

All notable changes to this project will be documented in this file.

## [v1.1.0] - 2025-12-22

### Added
- **Mixed Content Lists**: Added `content_type` support to lists. You can now have a "Weekend" list (named "weekend") that displays TV shows (`content_type`: "tv").
- **Shared Cache**: Poster and metadata cache is now shared across all lists that use the same Kodi host. This reduces redundant syncing.
- **Config Clarity**: Renamed the `type` field in `config.json` to `list_name` to better reflect its purpose (the identifier of the list in the UI).
- **Automatic Migrations**: Added a database migration system. Existing v1.0.0 databases will be automatically upgraded to the new schema on startup, preserving data.

### Changed
- **Database Schema**: The `lists` table now includes a `content_type` column and uses `name` instead of `type`.
- **Frontend**: Updated UI to use `contentType` for logic (e.g., "Add Show" vs "Add Movie" buttons) while keeping `listName` for routing.

### Breaking Changes
- **Configuration**: The `config.json` format has changed.
    - **Action Required**: Rename `"type"` to `"list_name"` in your `config.json`.
    - **Strict Typing**: The `content_type` field strictly accepts `"movie"` or `"tv"` (singular). Plural forms (e.g., `"movies"`) are not supported.

## [v1.0.0] - 2025-12-21

### Initial Release
- **Core Features**:
    - Manage multiple watchlists grouped by room/category.
    - Deep integration with Kodi JSON-RPC API.
    - Search and add Movies and TV Shows from your Kodi library.
    - Automatic metadata syncing (Posters, Plot, Ratings, Year).
    - Local caching of images for fast loading.
    - Drag-and-drop reordering of watchlist items.
