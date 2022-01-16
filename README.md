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
- nameserver records which do not resolve on the internet.
- mismatches between defined and actual nameserver records.
- duplicate managed zones in the organization.
- NS records without a matching managed zone in the organization.


# example

```bash
$ cloud-dns-integrity-checker -organization binx.com

2022/01/12 21:00:38 INFO: checking DNS nameserver integrity for organization binx.com
2022/01/12 21:00:39 INFO: checking nameserver integrity for xke.binx.com.
2022/01/12 21:00:39 ERROR: unconnected managed zone xke.binx.com. for domain xke-binx-com in project my-xke-project: lookup xke.binx.com. on 192.168.188.1:53: no such host
2022/01/12 21:00:40 ERROR: found another managed zone for the domain name xke.xebia.com. in project my-yadiya-project
2022/01/12 21:00:43 INFO: checking nameserver integrity for my.dns.binx.io.
2022/01/12 21:00:43 ERROR: unconnected managed zone my.dns.binx.io. for domain my-dns-zone in project my-project: lookup my.dns.binx.io. on 192.168.188.1:53: no such host
2022/01/12 21:00:44 INFO: checking nameserver integrity for google.binx.dev.
2022/01/12 21:48:51 ERROR: dangling nameserver somebody-else.google.binx.dev. in managed zone google-binx-dev of project my-project: lookup somebody-else.google.binx.dev. on [8.8.8.8]:53: server misbehaving
2022/01/12 21:00:45 INFO: checking nameserver integrity for gcp.binx.io.
2022/01/12 21:00:48 INFO: checking nameserver integrity for u.girlsday.fun.
2022/01/12 21:00:48 ERROR: unconnected managed zone u.girlsday.fun. for domain u-girlsday-fun in project my-other-project: lookup u.girlsday.fun. on 192.168.188.1:53: no such host
2022/01/12 21:00:55 ERROR: domain somebody-else.google.binx.dev. has a nameserver record in google-binx-dev, but there is no managed zone for it in this organization
2022/01/12 21:00:56 ERROR: domain mismatch.google.binx.dev. has a nameserver record in google-binx-dev, but there is no managed zone for it in this organization
```
