#!/bin/bash
# Email Checker Test Cases
# Run these commands to verify the tool is working correctly

BINARY="./bin/emailverify"
TARGET_IP="80.80.214.218"
TARGET_PORT="25"
VALID_EMAIL="info@garantbank.uz"
INVALID_EMAIL="nonexistent12345xyz@garantbank.uz"
DOMAIN="garantbank.uz"

echo "========================================"
echo "    EMAIL CHECKER TEST SUITE"
echo "========================================"
echo ""

# Test 1: Version check
echo "[TEST 1] Version check"
echo "Command: $BINARY version"
$BINARY version
echo ""

# Test 2: Help command
echo "[TEST 2] Help command"
echo "Command: $BINARY --help"
$BINARY --help
echo ""

# Test 3: Single email check (known valid)
echo "[TEST 3] Single email verification - VALID email"
echo "Command: $BINARY check $VALID_EMAIL -i $TARGET_IP -p $TARGET_PORT"
$BINARY check $VALID_EMAIL -i $TARGET_IP -p $TARGET_PORT
echo ""

# Test 4: Single email check with debug
echo "[TEST 4] Single email verification with DEBUG"
echo "Command: $BINARY check $VALID_EMAIL -i $TARGET_IP -p $TARGET_PORT -d"
$BINARY check $VALID_EMAIL -i $TARGET_IP -p $TARGET_PORT -d
echo ""

# Test 5: Single email check (invalid email)
echo "[TEST 5] Single email verification - INVALID email"
echo "Command: $BINARY check $INVALID_EMAIL -i $TARGET_IP -p $TARGET_PORT"
$BINARY check $INVALID_EMAIL -i $TARGET_IP -p $TARGET_PORT
echo ""

# Test 6: JSON output
echo "[TEST 6] JSON output format"
echo "Command: $BINARY check $VALID_EMAIL -i $TARGET_IP -p $TARGET_PORT --json"
$BINARY check $VALID_EMAIL -i $TARGET_IP -p $TARGET_PORT --json
echo ""

# Test 7: Syntax validation only (skip SMTP)
echo "[TEST 7] Syntax validation only (skip SMTP)"
echo "Command: $BINARY check $VALID_EMAIL --skip-smtp"
$BINARY check $VALID_EMAIL --skip-smtp
echo ""

# Test 8: Invalid syntax email
echo "[TEST 8] Invalid syntax email"
echo "Command: $BINARY check 'invalid-email-no-at-sign'"
$BINARY check 'invalid-email-no-at-sign'
echo ""

# Test 9: Domain check
echo "[TEST 9] Domain check"
echo "Command: $BINARY domain $DOMAIN"
$BINARY domain $DOMAIN
echo ""

# Test 10: Domain check with SPF and DMARC
echo "[TEST 10] Domain check with SPF and DMARC"
echo "Command: $BINARY domain $DOMAIN --check-spf --check-dmarc"
$BINARY domain $DOMAIN --check-spf --check-dmarc
echo ""

# Test 11: Domain check with catch-all detection
echo "[TEST 11] Domain check with catch-all detection"
echo "Command: $BINARY domain $DOMAIN --check-catchall -i $TARGET_IP"
$BINARY domain $DOMAIN --check-catchall
echo ""

# Test 12: Domain JSON output
echo "[TEST 12] Domain check JSON output"
echo "Command: $BINARY domain $DOMAIN --json"
$BINARY domain $DOMAIN --json
echo ""

# Test 13: Disposable email detection
echo "[TEST 13] Disposable email detection"
echo "Command: $BINARY check test@tempmail.com --skip-smtp"
$BINARY check test@tempmail.com --skip-smtp
echo ""

# Test 14: Role account detection
echo "[TEST 14] Role account detection"
echo "Command: $BINARY check admin@example.com --skip-smtp"
$BINARY check admin@example.com --skip-smtp
echo ""

# Test 15: Free provider detection
echo "[TEST 15] Free provider detection"
echo "Command: $BINARY check test@gmail.com --skip-smtp"
$BINARY check test@gmail.com --skip-smtp
echo ""

echo "========================================"
echo "    TEST SUITE COMPLETE"
echo "========================================"
