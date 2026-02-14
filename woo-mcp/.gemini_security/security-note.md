# Security Note: {{Title}}

## Date: {{Date}}

## Description:
{{Description}}

## Vulnerability Type:
{{Vulnerability Type}}

## Severity:
{{Critical/High/Medium/Low}}

## Recommendation:
{{Recommendation}}

---
# Security Note: Sensitive Data Exposure in Payment URL

## Date: 2026-02-14

## Description:
The payment link generation code includes the order key directly in the URL.
This can expose the order key through browser history, server logs, or referrer headers.
An attacker who obtains the order key may be able to gain unauthorized access to order information.

## Vulnerability Type:
Sensitive Data Exposure

## Severity:
Medium

## Recommendation:
Instead of passing the order key directly in the URL, consider using a one-time-use token or a session-based mechanism to authenticate the user for payment.
