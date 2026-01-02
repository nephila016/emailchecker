#!/bin/bash
# Bulk verification test cases

BINARY="./bin/emailverify"
TARGET_IP="80.80.214.218"
TARGET_PORT="25"
VALID_EMAIL="info@garantbank.uz"

# Create test email list
echo "[SETUP] Creating test email list..."
cat > test_emails.txt << 'EOF'
info@garantbank.uz
support@garantbank.uz
admin@garantbank.uz
nonexistent12345@garantbank.uz
randomuser99999@garantbank.uz
a.alimov@garantbank.uz
EOF

echo "Test emails created:"
cat test_emails.txt
echo ""

# Test 16: Bulk verification to CSV
echo "[TEST 16] Bulk verification - CSV output"
echo "Command: $BINARY bulk -f test_emails.txt -i $TARGET_IP -p $TARGET_PORT -o test_results.csv -w 1 -d 3 --jitter 1"
$BINARY bulk -f test_emails.txt -i $TARGET_IP -p $TARGET_PORT -o test_results.csv -w 1 -d 3 --jitter 1
echo ""
echo "CSV Results:"
cat test_results.csv
echo ""

# Test 17: Bulk verification to JSON
echo "[TEST 17] Bulk verification - JSON output"
echo "Command: $BINARY bulk -f test_emails.txt -i $TARGET_IP -p $TARGET_PORT -o test_results.json -w 1 -d 3"
$BINARY bulk -f test_emails.txt -i $TARGET_IP -p $TARGET_PORT -o test_results.json -w 1 -d 3
echo ""
echo "JSON Results:"
cat test_results.json
echo ""

# Test 18: Bulk with health checks
echo "[TEST 18] Bulk verification with health checks"
echo "Command: $BINARY bulk -f test_emails.txt -i $TARGET_IP -p $TARGET_PORT -o test_health.csv --health-email $VALID_EMAIL --health-interval 2 -w 1 -d 3"
$BINARY bulk -f test_emails.txt -i $TARGET_IP -p $TARGET_PORT -o test_health.csv --health-email $VALID_EMAIL --health-interval 2 -w 1 -d 3
echo ""

# Test 19: Bulk with debug mode
echo "[TEST 19] Bulk verification with debug"
echo "Command: $BINARY bulk -f test_emails.txt -i $TARGET_IP -p $TARGET_PORT -o test_debug.csv -w 1 -d 3 -d"
$BINARY bulk -f test_emails.txt -i $TARGET_IP -p $TARGET_PORT -o test_debug.csv -w 1 -d 3 -d
echo ""

# Cleanup
echo "[CLEANUP] Removing test files..."
rm -f test_emails.txt test_results.csv test_results.json test_health.csv test_debug.csv

echo "========================================"
echo "    BULK TESTS COMPLETE"
echo "========================================"
