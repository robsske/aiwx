#!/bin/bash
# AIWX Local Development Script
set -e

echo "🚀 Starting AIWX local development environment..."

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Start Docker services
echo -e "${YELLOW}Starting PostgreSQL and NATS...${NC}"
docker-compose up -d

# Wait for PostgreSQL
echo -e "${YELLOW}Waiting for PostgreSQL...${NC}"
until docker exec aiwx-postgres pg_isready -U aiwx 2>/dev/null; do
    sleep 1
done
echo -e "${GREEN}PostgreSQL is ready!${NC}"

# Build and run backend
echo -e "${YELLOW}Building backend...${NC}"
go build -o bin/aiwx ./cmd/aiwx

echo -e "${YELLOW}Starting backend on :8080...${NC}"
DATABASE_URL="postgres://aiwx:aiwx@localhost:5432/aiwx?sslmode=disable" \
JWT_SECRET="dev-secret-change-in-production" \
PORT=8080 \
./bin/aiwx &

BACKEND_PID=$!
echo -e "${GREEN}Backend started (PID: $BACKEND_PID)${NC}"

# Wait for backend
sleep 2

# Start UI
echo -e "${YELLOW}Starting UI on :3000...${NC}"
cd ui
npm install --silent
npm run dev &

UI_PID=$!
echo -e "${GREEN}UI started (PID: $UI_PID)${NC}"

echo ""
echo -e "${GREEN}✅ AIWX is running!${NC}"
echo ""
echo "  📊 Dashboard:  http://localhost:3000"
echo "  🔌 API:       http://localhost:8080/api/v1"
echo "  🔐 Login:     admin@aiwx.local / admin  (or SEED_ADMIN_PASSWORD)"
echo "  📈 Metrics:   http://localhost:8080/metrics"
echo "  🏥 Health:    http://localhost:8080/healthz"
echo "  🗄️  PostgreSQL: localhost:5432"
echo "  📨 NATS:      localhost:4222"
echo ""
echo "Press Ctrl+C to stop all services"
echo ""

# Cleanup on exit
trap "echo 'Stopping services...'; kill $BACKEND_PID $UI_PID 2>/dev/null; docker-compose down; exit" INT TERM

wait
