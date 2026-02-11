# Lusty – Backend API

Production-ready backend for an **18+** location-based companion discovery, booking, and paid communication platform (Tinder-style discovery + Uber-style live map). Built with **Go**, **Gin**, **Gorm**, **MySQL**, **Gorilla WebSockets**, and **Cloudinary**.

## Core principles

- **Strict adult-only access (18+)** – DOB verification and middleware
- **Consent-based interactions** – Paid access before communication
- **Location privacy** – Exact coordinates never exposed; distance and fuzzed map only
- **Real-time presence** – Online/offline/busy/in-session and live map updates via WebSockets

## Tech stack

- **Go 1.22+**, **Gin**, **Gorm**, **MySQL**
- **Gorilla WebSockets** – Map stream + chat
- **JWT** (access + refresh), **bcrypt**, optional **Google OAuth**
- **Cloudinary** – Image/video upload with optimization (q_auto, f_auto, resize)
- **Clean architecture** – Config, models, repository, service, handler, middleware, WS hubs

## Setup

### Prerequisites

- Go 1.22+
- MySQL 8+
- (Optional) Cloudinary account for media

### Environment variables

Create a `.env` or export before running:

```bash
# Server
PORT=8080
ENV=development
READ_TIMEOUT=10s
WRITE_TIMEOUT=10s

# Database
DB_DSN=user:password@tcp(localhost:3306)/lusty?charset=utf8mb4&parseTime=True&loc=Local
DB_MAX_IDLE=10
DB_MAX_OPEN=100
DB_CONN_MAX_LIFETIME=1h

# JWT
JWT_ACCESS_SECRET=your-access-secret
JWT_REFRESH_SECRET=your-refresh-secret
JWT_ACCESS_EXPIRY=15m
JWT_REFRESH_EXPIRY=168h
JWT_ISSUER=lusty

# OAuth (optional)
GOOGLE_CLIENT_ID=
GOOGLE_CLIENT_SECRET=
GOOGLE_REDIRECT_URL=http://localhost:8080/api/v1/auth/google/callback

# Cloudinary (use env in production; defaults exist for dev)
CLOUDINARY_CLOUD_NAME=your_cloud
CLOUDINARY_API_KEY=your_key
CLOUDINARY_API_SECRET=your_secret

# Location & map
MAP_UPDATE_INTERVAL_SEC=5
LOCATION_FUZZ_METERS=100
MIN_AGE=18

# Payments (webhook verification)
PAYMENT_WEBHOOK_SECRET=
PAYMENT_EXPIRY=30m
```

### Run

```bash
# Install dependencies
go mod download

# Auto-migrate DB (run on startup)
# Migrations are Gorm AutoMigrate (see internal/database/database.go).

# Start server
go run ./cmd/server
```

Server listens on `:8080` by default.

## API overview

- **Auth**: `POST /api/v1/auth/register`, `POST /api/v1/auth/login`, `POST /api/v1/auth/logout` (Bearer), `POST /api/v1/auth/refresh`, `GET /api/v1/auth/google` (redirect), `GET /api/v1/auth/google/callback`
- **Discovery (Tinder-style)**: `GET /api/v1/discover?lat=&lng=&radius_km=&min_age=&max_age=&online_only=&sort=&limit=&offset=`
- **Companion profile**: `GET /api/v1/companions/:id`, `PUT /api/v1/companions/profile` (companion), `POST /api/v1/companions/media` (companion)
- **Companion pricing**: `GET/POST /api/v1/companions/pricing`, `PUT/DELETE /api/v1/companions/pricing/:id` (companion)
- **Boost**: `POST /api/v1/companions/boost` (body: `boost_type`: 1h|24h|72h, optional `payment_reference`) (companion)
- **Location**: `PATCH /api/v1/me/location`, `GET /api/v1/me/location`
- **Presence**: `PATCH /api/v1/me/presence`, `GET /api/v1/me/presence` (setting ONLINE as companion notifies favoriting clients)
- **Favorites**: `GET /api/v1/me/favorites`, `POST /api/v1/favorites/:companion_id`, `DELETE /api/v1/favorites/:companion_id`
- **Interactions**: `POST /api/v1/interactions` (body: `companion_id`, `interaction_type`, `payment_id` or `payment_reference`, optional `duration_minutes`), `GET /api/v1/me/interactions` (list), `POST /api/v1/interactions/:id/accept`, `POST /api/v1/interactions/:id/reject` (companion)
- **Chat**: `GET /api/v1/me/interactions/:interaction_id/messages?limit=&offset=`, WebSocket `GET /ws/chat?token=&interaction_id=` (send JSON `{ "type": "message", "content": "", "media_url": "" }`)
- **Video signaling**: WebSocket `GET /ws/video?token=&interaction_id=` (send `{ "type": "offer"|"answer"|"ice", "payload": ... }`)
- **Payment webhook**: `POST /api/v1/webhooks/payment` (body: `reference`, `status`; optional `X-Webhook-Signature` when `PAYMENT_WEBHOOK_SECRET` set)
- **WebSocket map**: `GET /ws/map?token=<access_token>` – clients receive fuzzed companion markers; companions push location via `PATCH /api/v1/me/location`

Protected routes require `Authorization: Bearer <access_token>` and (where applied) 18+ middleware. Login/register set presence to ONLINE; logout sets OFFLINE. Audit logs are written for auth and report actions and payment completion.

## Location logic

- **Storage**: `user_locations` with `latitude`, `longitude`, `accuracy_meters`, `is_location_visible`, `last_updated_at`. Indexed for bounding-box queries.
- **Discovery**: Bounding box (lat/lng ± radius) then **Haversine** in app to filter by exact `radius_km`. Results expose **distance_km** only (no exact coordinates).
- **Live map**: Companions update location via `PATCH /me/location`. Backend **fuzzes** coordinates (configurable meters) and pushes to **MapHub**; clients subscribed to `GET /ws/map` receive `{ type: "markers", markers: [...] }`. Update interval throttled by client (e.g. every 5–10s).
- **Privacy**: Exact coordinates never returned to other users; only distance and fuzzed map positions.

## Payments & M-Pesa (TheLiberec)

- **M-Pesa (Kenya)**: Uses [TheLiberec Card API](https://card-api.theliberec.com). Token per transaction via `POST /api/v1/merchants/login`, then STK push via `POST /api/v1/transactions/mpesa`.
- **Flow**:
  1. Client: `POST /api/v1/payments/mpesa/initiate` with `companion_id`, `interaction_type`, `amount_kes`, `customer_phone`, `customer_first_name`, `customer_last_name`, `customer_email`
  2. Backend creates Payment (PENDING) + InteractionRequest (PENDING), sends STK push
  3. User pays on phone; TheLiberec calls `POST /api/v1/webhooks/mpesa` with `merchant_order_id` (= our order_id), `status`
  4. When `status=COMPLETED`: payment marked done, interaction auto-accepted, ChatSession created → **chat and video unlocked**
- **Distance tracking (no map)**: `GET /api/v1/me/interactions/:id/distance` returns `distance_km` between client and companion so the client can see "the lady is coming" as distance decreases. Both must have location updated.

## WebSockets & video signaling

- **Map**: `GET /ws/map?token=...` – authenticated; server sends initial markers and pushes updates when companions move (fuzzed). No client coordinates sent to other users.
- **Chat**: One room per accepted interaction (design in `internal/ws/chat_hub.go`). Authenticate, then join room by `interaction_id`; messages persisted via `ChatMessage` and broadcast to the room.
- **Video**: Backend handles **session authorization**, **time tracking**, and **signaling** over WebSockets (offer/answer/ICE). Design is ready for a future SFU (e.g. LiveKit, Mediasoup) for media relay.

## Cloudinary

- **Upload**: Companion images/videos via `POST /api/v1/companions/media` (multipart). Uses **Cloudinary Go SDK** with **eager transformations**: `q_auto,f_auto,w_800,c_fill` for images, `q_auto:low,f_auto,w_1280` for video so the frontend receives optimized URLs and faster loads.
- **Config**: Set `CLOUDINARY_*` in production; defaults in code are for local dev only.

## Safety & moderation

- **18+**: Enforced at signup (DOB) and via **AdultOnly** middleware on sensitive routes.
- **Block / report**: `Block` and `Report` models and repos; wire to handlers and exclude blocked users from discovery and messaging.
- **Rate limiting**: In-memory limiter per IP on the main router.
- **Audit logs**: `AuditLog` model and repo; call from critical actions (login, payment, report).
- **Media moderation**: Hook Cloudinary moderation or external APIs when saving `CompanionMedia`; flag for admin review.

## Project layout

```
cmd/server/          # Entry point
config/              # Env-based config
internal/
  auth/               # JWT issue/parse
  database/           # Gorm connect + AutoMigrate
  domain/             # Constants (roles, statuses)
  handler/            # HTTP handlers
  middleware/          # Auth, 18+, rate limit
  models/             # Gorm models
  repository/         # Data access
  router/             # Gin routes
  service/            # Auth, etc.
  ws/                 # Map + chat hubs, upgrade
pkg/
  cloudinary/         # Upload + optimized URLs
  location/           # Haversine, fuzz
  payment/            # Provider interface + stub
```

## License

Proprietary. Use according to your terms.
# metchy-backend
