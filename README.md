# cloud-dns-integrity-checker

Checks the integrity of your Cloud DNS managed zones in your organization

# Options
```
  -organization string
        to check the DNS integrity of
  -use-default-credentials
        to authenticate against GCP, otherwise uses gcloud credentials
```

# Description
Checks the correctness of all the nameserver records (NS) in your GCP organization.

it will report:
- NS records defined which did not resolve in real life.
- report any mismatch in resolved nameserver records.

