package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"regexp"
	"sort"
	"strings"
	"time"

	asset "cloud.google.com/go/asset/apiv1"
	assetpb "google.golang.org/genproto/googleapis/cloud/asset/v1"

	"github.com/binxio/gcloudconfig"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/cloudresourcemanager/v1beta1"
	"google.golang.org/api/dns/v1"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

var dnsManagedZoneRegex = regexp.MustCompile(`^//dns.googleapis.com/projects/([^/]*)/managedZones/(.*)$`)

// getProjectIDAndName returns the project id and name from the managedZonedAssetName. if it matches
// the regular expression `^//dns.googleapis.com/projects/([^/]*)/managedZones/(.*)$`.
func getProjectIDAndName(managedZoneAssetName string) (string, string, error) {
	match := dnsManagedZoneRegex.FindAllStringSubmatch(managedZoneAssetName, -1)
	if match == nil {
		return "", "", fmt.Errorf("%s is not a managed zone asset name", managedZoneAssetName)
	}

	projectID := match[0][1]
	managedZoneID := match[0][2]
	return projectID, managedZoneID, nil
}

// CloudDNSIntegrityChecker represents the command to check the integrity of the Cloud DNS in your organization
type CloudDNSIntegrityChecker struct {
	Organization          string
	UseDefaultCredentials bool
	IncludePrivateZones   bool
	credentials           *google.Credentials
	organization          *cloudresourcemanager.Organization
	resolver              *net.Resolver
	managedZones          map[string]*dns.ManagedZone
}

// getCredentials get the google credentials and stores them in credentials. If UseDefaultCredentials is set
// it will use the default credentials, otherwise it will use the gcloud credentials of the activate configuration.
func (c *CloudDNSIntegrityChecker) getCredentials(ctx context.Context) error {
	var err error

	if c.UseDefaultCredentials {
		c.credentials, err = google.FindDefaultCredentials(ctx, "https://www.googleapis.com/auth/cloud-platform")
	} else {
		c.credentials, err = gcloudconfig.GetCredentials("")
	}
	return err
}

// selectGoogleOrganization sets organization based on the value specified in Organization. It
// will match both on display name and the organization id. If none is specified and there is only one organization
// active, it will select the default org.
func (c *CloudDNSIntegrityChecker) selectGoogleOrganization(ctx context.Context) error {
	svc, err := cloudresourcemanager.NewService(ctx, option.WithTokenSource(c.credentials.TokenSource))
	if err != nil {
		return err
	}

	organisations, err := svc.Organizations.List().Do()
	if err != nil {
		return err
	}

	if c.Organization == "" {
		if len(organisations.Organizations) != 1 {
			return fmt.Errorf("please specify an organization to check")
		}
		c.organization = organisations.Organizations[0]

	} else {
		for _, org := range organisations.Organizations {
			if org.DisplayName == c.Organization || org.OrganizationId == c.Organization || org.Name == c.Organization {
				c.organization = org
				break
			}
		}
		if c.organization == nil {
			return fmt.Errorf("you do not have access to the organization %s", c.Organization)
		}
	}
	return nil
}

// loadManagedZones will load all DNS managed zones of the organization in managedZones.
// If IncludePrivateZones is false, it will only load public DNS managed zones.
// It will skip managed zones which cannot be retrieved, while printing an error log message.
func (c *CloudDNSIntegrityChecker) loadManagedZones(ctx context.Context) error {
	assetService, err := asset.NewClient(ctx, option.WithCredentials(c.credentials))
	if err != nil {
		return err
	}
	defer assetService.Close()

	dnsService, err := dns.NewService(ctx, option.WithCredentials(c.credentials))
	if err != nil {
		return err
	}

	c.managedZones = make(map[string]*dns.ManagedZone, 0)

	assets := assetService.ListAssets(ctx, &assetpb.ListAssetsRequest{
		Parent:     c.organization.Name,
		AssetTypes: []string{"dns.googleapis.com/ManagedZone"},
	})
	for {
		asset, err := assets.Next()
		if err == iterator.Done {
			return nil
		}
		if err != nil {
			return err
		}

		projectID, managedZoneID, err := getProjectIDAndName(asset.Name)
		if err != nil {
			log.Printf("ERROR: %s", err)
			continue
		}
		zone, err := dnsService.ManagedZones.Get(projectID, managedZoneID).Do()
		if err != nil {
			log.Printf("ERROR: failed to get %s, %s", asset.Name, err)
			continue
		}
		if c.IncludePrivateZones || zone.PrivateVisibilityConfig != nil {
			continue
		}

		c.managedZones[asset.Name] = zone
	}
}

// compareNameserverRecords returns true when resourceRecordSet matches the nameServers records, otherwise false.
func compareNameserverRecords(resourceRecordSet *dns.ResourceRecordSet, nameServers []*net.NS) bool {
	definedNameServers := make(map[string]bool, len(resourceRecordSet.Rrdatas))

	if len(resourceRecordSet.Rrdatas) != len(nameServers) {
		return false
	}

	for _, nameServer := range resourceRecordSet.Rrdatas {
		definedNameServers[nameServer] = true
	}

	for _, nameServer := range nameServers {
		if _, exists := definedNameServers[nameServer.Host]; !exists {
			return false
		}
	}
	return true
}

// Check checks the integrity of your managedZones in your organization.
func (c *CloudDNSIntegrityChecker) Check(ctx context.Context) error {
	managedZones := make(map[string]*dns.ManagedZone)
	subDomainReferrals := make(map[string]*dns.ManagedZone)

	dnsService, err := dns.NewService(ctx, option.WithCredentials(c.credentials))
	if err != nil {
		return err
	}
	log.Printf("INFO: checking DNS nameserver integrity for organization %s", c.organization.DisplayName)

	for assetName, zone := range c.managedZones {
		projectID, managedZoneID, err := getProjectIDAndName(assetName)
		if err != nil {
			return err
		}

		if _, exists := managedZones[zone.DnsName]; exists {
			log.Printf("ERROR: found another managed zone for the domain name %s in project %s",
				zone.DnsName, projectID)
		}

		response, err := dnsService.ResourceRecordSets.List(projectID, managedZoneID).Do()
		if err != nil {
			return fmt.Errorf("ERROR: failed to list resource record sets of %s in %s, %s",
				managedZoneID, projectID, err)
		}
		for _, resourceRecordSet := range response.Rrsets {
			if resourceRecordSet.Type == "NS" {
				log.Printf("INFO: checking nameserver integrity for %s", resourceRecordSet.Name)

				if resourceRecordSet.Name != zone.DnsName {
					subDomainReferrals[resourceRecordSet.Name] = zone
				}

				nameservers, err := c.resolver.LookupNS(ctx, resourceRecordSet.Name)
				if err != nil {
					if resourceRecordSet.Name == zone.DnsName {
						log.Printf("ERROR: unresolved nameservers for domain %s of managed zone %s in project %s: %s",
							resourceRecordSet.Name, zone.Name, projectID, err)
					} else {
						log.Printf("ERROR: unresolved nameservers for domain %s in managed zone %s of project %s: %s",
							resourceRecordSet.Name, zone.Name, projectID, err)
					}
					continue
				}
				if !compareNameserverRecords(resourceRecordSet, nameservers) {
					ns := make([]string, len(nameservers))
					for i, n := range nameservers {
						ns[i] = n.Host
					}
					sort.Strings(ns)

					log.Printf("ERROR: mismatch in nameserver configuration for managed zone %s for domain %s in project %s. expected\n\t%s\ngot\n\t%s",
						resourceRecordSet.Name, zone.Name, projectID, strings.Join(resourceRecordSet.Rrdatas, "\n\t"),
						strings.Join(ns, "\n\t"))
				}
			}
		}
	}
	for subDomain, parentZone := range subDomainReferrals {
		if _, exists := managedZones[subDomain]; !exists {
			log.Printf("ERROR: domain %s has a nameserver record in %s, but there is no managed zone for it in this organization",
				subDomain, parentZone.Name)
		}
	}
	return nil
}

func main() {
	var cmd CloudDNSIntegrityChecker
	ctx := context.Background()
	cmd.resolver = &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{
				Timeout: time.Millisecond * time.Duration(10000),
			}
			return d.DialContext(ctx, network, "8.8.8.8:53")
		},
	}

	flag.StringVar(&cmd.Organization, "organization", "", "to check the DNS integrity of")
	flag.BoolVar(&cmd.UseDefaultCredentials, "use-default-credentials", false, "to authenticate against GCP")
	flag.Parse()

	if err := cmd.getCredentials(ctx); err != nil {
		log.Fatalf("%s", err)
	}
	if err := cmd.selectGoogleOrganization(ctx); err != nil {
		log.Fatalf("%s", err)
	}
	if err := cmd.loadManagedZones(ctx); err != nil {
		log.Fatalf("%s", err)
	}

	if err := cmd.Check(ctx); err != nil {
		log.Fatalf("%s", err)
	}
}
