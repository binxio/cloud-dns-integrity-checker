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

		response, err := dnsService.ResourceRecordSets.List(projectID, managedZoneID).Do()
		if err != nil {
			log.Fatalf("%s", err)
		}
		for _, resourceRecordSet := range response.Rrsets {
			if resourceRecordSet.Type == "NS" {
				nameserver, err := googleDNS.LookupNS(ctx, resourceRecordSet.Name)
				if err != nil {
					if resourceRecordSet.Name == zone.DnsName {
						log.Printf("NOTICE: remove managed zone %s (%s) in project %s: %s",
							resourceRecordSet.Name, zone.Name, projectID, err)
					} else {
						log.Printf("NOTICE: remove dangling %s NS from managed zone %s in project %s: %s",
							resourceRecordSet.Name, zone.Name, projectID, err)
					}
					continue
				}
				log.Printf("INFO: checking nameserver integrity for %s", resourceRecordSet.Name)

				definedNameServers := make(map[string]bool, len(resourceRecordSet.Rrdatas))
				actualNameServers := make(map[string]bool, len(resourceRecordSet.Rrdatas))

				for _, nameServer := range resourceRecordSet.Rrdatas {
					definedNameServers[nameServer] = true
				}

				for _, nameServer := range nameserver {
					actualNameServers[nameServer.Host] = true
					if _, exists := definedNameServers[nameServer.Host]; !exists {
						log.Printf("NOTICE: remove '%s' from NS record for domain %s in managed zone %s in project %s",
							nameServer.Host, resourceRecordSet.Name, zone.Name, projectID)
					}
				}

				for nameServer := range definedNameServers {
					if _, exists := actualNameServers[nameServer]; !exists {
						log.Printf("NOTICE: add '%s' to NS record for domain %s in managed zone %s in project %s",
							nameServer, resourceRecordSet.Name, zone.Name, projectID)
					}
				}
			}
		}
	}
}
