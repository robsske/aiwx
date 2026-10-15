#!/bin/bash
# Quick smoke test for AIWX
set -e

echo "🧪 Running AIWX smoke tests..."

GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

API_URL="http://localhost:8080"

# Test health endpoint
echo -e "${YELLOW}Testing health endpoint...${NC}"
HEALTH=$(curl -s "$API_URL/healthz")
if echo "$HEALTH" | grep -q '"status":"ok"'; then
    echo -e "${GREEN}✅ Health check passed${NC}"
else
    echo -e "${RED}❌ Health check failed${NC}"
    exit 1
fi

# Test API root
echo -e "${YELLOW}Testing API root...${NC}"
API_ROOT=$(curl -s "$API_URL/api/v1/")
if echo "$API_ROOT" | grep -q '"name":"AIWX API"'; then
    echo -e "${GREEN}✅ API root passed${NC}"
else
    echo -e "${RED}❌ API root failed${NC}"
    exit 1
fi

# Test metrics endpoint
echo -e "${YELLOW}Testing metrics endpoint...${NC}"
METRICS=$(curl -s "$API_URL/metrics" | head -5)
if echo "$METRICS" | grep -q "aiwx_"; then
    echo -e "${GREEN}✅ Metrics endpoint passed${NC}"
else
    echo -e "${RED}❌ Metrics endpoint failed${NC}"
    exit 1
fi

# Test auth required
echo -e "${YELLOW}Testing auth enforcement...${NC}"
AUTH_STATUS=$(curl -s -o /dev/null -w "%{http_code}" "$API_URL/api/v1/organizations")
if [ "$AUTH_STATUS" = "401" ]; then
    echo -e "${GREEN}✅ Auth enforcement passed${NC}"
else
    echo -e "${RED}❌ Auth enforcement failed (got $AUTH_STATUS)${NC}"
    exit 1
fi

echo ""
echo -e "${GREEN}✅ All smoke tests passed!${NC}"
echo ""
echo "Try these commands:"
echo ""
echo "  # View health"
echo "  curl http://localhost:8080/healthz"
echo ""
echo "  # View metrics"
echo "  curl http://localhost:8080/metrics | head -20"
echo ""
echo "  # Open UI"
echo "  open http://localhost:3000"
