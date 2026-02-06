#!/bin/bash
set -e

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

echo "=================================="
echo "Valiant Kubernetes Collector Tests"
echo "=================================="

# Check if Docker is running
if ! docker info > /dev/null 2>&1; then
    echo -e "${RED}Error: Docker is not running${NC}"
    exit 1
fi

# Start PostgreSQL container
echo -e "${YELLOW}Starting PostgreSQL container...${NC}"
docker run -d \
  --name valiant-test-postgres-$\
  -e POSTGRES_USER=testuser \
  -e POSTGRES_PASSWORD=testpass \
  -e POSTGRES_DB=valiant_test \
  -p 5432:5432 \
  postgres:15-alpine > /dev/null

# Wait for PostgreSQL to be ready
echo -e "${YELLOW}Waiting for PostgreSQL to be ready...${NC}"
sleep 3
for i in {1..30}; do
    if docker exec valiant-test-postgres-$\
 pg_isready -U testuser > /dev/null 2>&1; then
        echo -e "${GREEN}PostgreSQL is ready!${NC}"
        break
    fi
    if [ $i -eq 30 ]; then
        echo -e "${RED}PostgreSQL failed to start${NC}"
        docker stop valiant-test-postgres-$\

        docker rm valiant-test-postgres-$\

        exit 1
    fi
    sleep 1
done

# Set environment variables
export DB_HOST=localhost
export DB_PORT=5432
export DB_USER=testuser
export DB_PASSWORD=testpass
export DB_NAME=valiant_test

# Run tests
echo -e "${YELLOW}Running tests...${NC}"
echo ""
if go test ./... -v -count=1; then
    echo ""
    echo -e "${GREEN}✓ All tests passed!${NC}"
    EXIT_CODE=0
else
    echo ""
    echo -e "${RED}✗ Tests failed!${NC}"
    EXIT_CODE=1
fi

# Cleanup
echo -e "${YELLOW}Cleaning up...${NC}"
docker stop valiant-test-postgres-$\
> /dev/null
docker rm valiant-test-postgres-$\
> /dev/null

echo -e "${GREEN}Cleanup complete${NC}"
exit $EXIT_CODE
