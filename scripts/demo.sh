#!/bin/sh
set -eu

curl --fail-with-body --request POST http://localhost:8080/payments/events \
  --header 'Content-Type: application/json' \
  --data '{"payment_id":"pay_2026_09_03_01","account_id":"acct_42","amount_minor":125000,"currency":"USD","kind":"authorized"}'
