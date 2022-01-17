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
$ cloud-dns-integrity-checker -organization xebia.com

2022/01/17 21:17:21 INFO: checking DNS nameserver integrity for organization xebia.com
2022/01/17 21:17:22 INFO: checking nameserver integrity for somebody-else.google.binx.dev.
2022/01/17 21:17:22 ERROR: mismatch in nameserver configuration for managed zone somebody-else.google.binx.dev. for domain somebody-else-google-binx-dev in project speeltuin-mvanholsteijn. expected
	ns-cloud-c1.googledomains.com.
	ns-cloud-c2.googledomains.com.
	ns-cloud-c3.googledomains.com.
	ns-cloud-c4.googledomains.com.
got
	ns-cloud-b1.googledomains.com.
	ns-cloud-b2.googledomains.com.
	ns-cloud-b3.googledomains.com.
	ns-cloud-b4.googledomains.com.
2022/01/17 21:17:22 INFO: checking nameserver integrity for google.binx.dev.
2022/01/17 21:17:22 INFO: checking nameserver integrity for mismatch.google.binx.dev.
2022/01/17 21:17:23 ERROR: unresolved nameservers for domain mismatch.google.binx.dev. in managed zone google-binx-dev of project speeltuin-mvanholsteijn: lookup mismatch.google.binx.dev. on [fd00::2e91:abff:fea1:b7b6]:53: server misbehaving
2022/01/17 21:17:23 INFO: checking nameserver integrity for somebody-else.google.binx.dev.
2022/01/17 21:17:23 INFO: checking nameserver integrity for xke.xebia.com.
2022/01/17 21:17:23 ERROR: unresolved nameservers for domain xke.xebia.com. of managed zone xke-xebia-com in project speeltuin-mvanholsteijn-2: lookup xke.xebia.com. on 192.168.188.1:53: no such host
2022/01/17 21:17:23 INFO: checking nameserver integrity for ttn.binx.io.
2022/01/17 21:17:23 INFO: checking nameserver integrity for playf.internal.
2022/01/17 21:17:23 ERROR: unresolved nameservers for domain playf.internal. of managed zone test-zone-internal in project playground-fchyla: lookup playf.internal. on 192.168.188.1:53: no such host
2022/01/17 21:17:23 INFO: checking nameserver integrity for xke.xebia.com.
2022/01/17 21:17:23 ERROR: unresolved nameservers for domain xke.xebia.com. of managed zone xke-xebia-com in project xke-nxt: lookup xke.xebia.com. on 192.168.188.1:53: no such host
2022/01/17 21:17:24 INFO: checking nameserver integrity for mismatch.google.binx.dev.
2022/01/17 21:17:24 ERROR: unresolved nameservers for domain mismatch.google.binx.dev. of managed zone mismatch-google-binx-dev in project speeltuin-mvanholsteijn: lookup mismatch.google.binx.dev. on [fd00::2e91:abff:fea1:b7b6]:53: server misbehaving
2022/01/17 21:17:24 INFO: checking nameserver integrity for my.dns.binx.io.
2022/01/17 21:17:24 ERROR: unresolved nameservers for domain my.dns.binx.io. of managed zone my-dns-zone in project speeltuin-mvanholsteijn: lookup my.dns.binx.io. on 192.168.188.1:53: no such host
2022/01/17 21:17:25 INFO: checking nameserver integrity for u.girlsday.fun.
2022/01/17 21:17:25 ERROR: unresolved nameservers for domain u.girlsday.fun. of managed zone u-girlsday-fun in project girlsday: lookup u.girlsday.fun. on 192.168.188.1:53: no such host
2022/01/17 21:17:25 ERROR: domain mismatch.google.binx.dev. has a nameserver record in google-binx-dev, but there is no managed zone for it in this organization
2022/01/17 21:17:25 ERROR: domain somebody-else.google.binx.dev. has a nameserver record in google-binx-dev, but there is no managed zone for it in this organization
```
