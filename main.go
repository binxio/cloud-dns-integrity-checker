// Sample asset-quickstart exports assets to given path.
package main

import (
	"context"
	"flag"
	"log"
	"net"
	"regexp"
	"time"

	asset "cloud.google.com/go/asset/apiv1"

	"github.com/binxio/gcloudconfig"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/cloudresourcemanager/v1beta1"
	"google.golang.org/api/dns/v1"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
	assetpb "google.golang.org/genproto/googleapis/cloud/asset/v1"
)

var googleDNS = &net.Resolver{
	PreferGo: true,
	Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
		d := net.Dialer{
			Timeout: time.Millisecond * time.Duration(10000),
		}
		return d.DialContext(ctx, network, "8.8.8.8:53")
	},
}

func main() {
	var err error
	var credentials *google.Credentials
	var organization *cloudresourcemanager.Organization

	ctx := context.Background()
	scopes := []string{"https://www.googleapis.com/auth/cloud-platform"}
	dnsManagedZoneRegex := regexp.MustCompile(`^//dns.googleapis.com/projects/([^/]*)/managedZones/(.*)$`)
	managedZones := make(map[string]*dns.ManagedZone)
	subDomainReferrals := make(map[string]*dns.ManagedZone)

	organizationName := flag.String("organization", "", "to check the DNS integrity of")
	useDefaultCredentials := flag.Bool("use-default-credentials", false, "to authenticate against GCP")

	flag.Parse()

	if *useDefaultCredentials {
		credentials, err = google.FindDefaultCredentials(ctx, scopes...)
	} else {
		credentials, err = gcloudconfig.GetCredentials("")
	}

	if err != nil {
		log.Fatalf("ERROR: could not obtain credentials, %s", err)
	}

	svc, err := cloudresourcemanager.NewService(ctx, option.WithTokenSource(credentials.TokenSource))
	if err != nil {
		log.Fatal(err)
	}

	orgs, err := svc.Organizations.List().Do()
	if err != nil {
		log.Fatal(err)
	}

	if *organizationName == "" {
		if len(orgs.Organizations) != 1 {
			log.Fatalf("ERROR: please specify an organization to check")
		}
		organization = orgs.Organizations[0]
	} else {
		for _, org := range orgs.Organizations {
			if org.DisplayName == *organizationName || org.OrganizationId == *organizationName || org.Name == *organizationName {
				organization = org
				break
			}
		}
		if organization == nil {
			log.Fatalf("you do not have access to the organization %s", *organizationName)
		}
	}

	log.Printf("INFO: checking DNS nameserver integrity for organization %s", organization.DisplayName)

	dnsService, err := dns.NewService(ctx, option.WithCredentials(credentials))
	if err != nil {
		log.Fatal(err)
	}

	assetService, err := asset.NewClient(ctx, option.WithCredentials(credentials))
	if err != nil {
		log.Fatal(err)
	}
	defer assetService.Close()

	assets := assetService.ListAssets(ctx, &assetpb.ListAssetsRequest{
		Parent:     organization.Name,
		AssetTypes: []string{"dns.googleapis.com/ManagedZone"},
	})
	for {
		asset, err := assets.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			log.Fatalf("%s", err)
		}

		match := dnsManagedZoneRegex.FindAllStringSubmatch(asset.Name, -1)
		if match == nil {
			log.Fatalf("%s is not a managed zone URL", asset.Name)
		}

		projectID := match[0][1]
		managedZoneID := match[0][2]
		zone, err := dnsService.ManagedZones.Get(projectID, managedZoneID).Do()
		if err != nil {
			log.Printf("ERROR: %s, %s", asset.Name, err)
			continue
		}
		if zone.PrivateVisibilityConfig != nil {
			continue
		}

		if _, exists := managedZones[zone.DnsName]; exists {
			log.Printf("ERROR: found another managed zone for the domain name %s in project %s",
				zone.DnsName, projectID)
		}
		managedZones[zone.DnsName] = zone

		response, err := dnsService.ResourceRecordSets.List(projectID, managedZoneID).Do()
		if err != nil {
			log.Fatalf("%s", err)
		}
		for _, resourceRecordSet := range response.Rrsets {
			if resourceRecordSet.Type == "NS" {
				log.Printf("INFO: checking nameserver integrity for %s", resourceRecordSet.Name)

				nameserver, err := googleDNS.LookupNS(ctx, resourceRecordSet.Name)
				if err != nil {
					if resourceRecordSet.Name == zone.DnsName {
						log.Printf("ERROR: unconnected managed zone %s for domain %s in project %s: %s",
							resourceRecordSet.Name, zone.Name, projectID, err)
					} else {
						log.Printf("ERROR: dangling nameserver %s in managed zone %s of project %s: %s",
							resourceRecordSet.Name, zone.Name, projectID, err)
					}
					continue
				}

				if resourceRecordSet.Name != zone.DnsName {
					subDomainReferrals[resourceRecordSet.Name] = zone
				}

				definedNameServers := make(map[string]bool, len(resourceRecordSet.Rrdatas))
				actualNameServers := make(map[string]bool, len(resourceRecordSet.Rrdatas))

				for _, nameServer := range resourceRecordSet.Rrdatas {
					definedNameServers[nameServer] = true
				}

				for _, nameServer := range nameserver {
					actualNameServers[nameServer.Host] = true
					if _, exists := definedNameServers[nameServer.Host]; !exists {
						log.Printf("ERROR: incorrect nameserver '%s' for domain %s. It does not exist in managed zone %s of project %s",
							nameServer.Host, resourceRecordSet.Name, zone.Name, projectID)
					}
				}

				for nameServer := range definedNameServers {
					if _, exists := actualNameServers[nameServer]; !exists {
						log.Printf("ERROR: missing nameserver '%s' for domain %s. It does exist in managed zone %s of project %s",
							nameServer, resourceRecordSet.Name, zone.Name, projectID)
					}
				}
			}
		}

		for subDomain, parentZone := range subDomainReferrals {
			if _, exists := managedZones[subDomain]; !exists {
				log.Printf("ERROR: domain %s has a nameserver record, but there is no managed zone for it in this organization",
					subDomain, parentZone.Name)
			}
		}
	}
}
