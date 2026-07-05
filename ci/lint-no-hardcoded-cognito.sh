#!/bin/bash
# Fail if any HTML file contains hardcoded Cognito URLs in HTML attributes.
# JS-generated URLs (template strings from variables) are fine.
if grep -rn 'href="https\?://[^"]*amazoncognito\.com' prototype/backend/web/*.html; then
  echo "ERROR: Hardcoded Cognito URLs found in HTML attributes. All Cognito URLs must be JS-generated."
  exit 1
fi
echo "OK: No hardcoded Cognito URLs"
