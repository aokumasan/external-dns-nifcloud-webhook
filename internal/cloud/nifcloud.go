package cloud

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/nifcloud/nifcloud-sdk-go/nifcloud"
	"github.com/nifcloud/nifcloud-sdk-go/service/dns"
	"github.com/nifcloud/nifcloud-sdk-go/service/dns/types"
	"sigs.k8s.io/external-dns/endpoint"
	"sigs.k8s.io/external-dns/plan"
	"sigs.k8s.io/external-dns/provider"
)

const defaultTTL = 3600

type nifcloudProvider struct {
	provider.BaseProvider
	client *dns.Client
}

func NewNifcloudProvider(accessKeyID, secretAccessKey string) (provider.Provider, error) {
	if accessKeyID == "" {
		return nil, fmt.Errorf("access key id is required")
	}

	if secretAccessKey == "" {
		return nil, fmt.Errorf("secret access key is required")
	}

	cfg := nifcloud.NewConfig(
		accessKeyID, secretAccessKey, "jp-east-1",
	)

	cfg.ClientLogMode = aws.LogRequestWithBody | aws.LogResponseWithBody

	return &nifcloudProvider{
		client: dns.NewFromConfig(cfg),
	}, nil
}

func (n *nifcloudProvider) Records(ctx context.Context) ([]*endpoint.Endpoint, error) {
	timeoutCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	zones, err := n.client.ListHostedZones(timeoutCtx, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch zone list: %w", err)
	}

	endpoints := []*endpoint.Endpoint{}
	for _, zone := range zones.HostedZones {
		records, err := n.client.ListResourceRecordSets(timeoutCtx, &dns.ListResourceRecordSetsInput{ZoneID: zone.Name})
		if err != nil {
			return nil, fmt.Errorf("failed to fetch records in zone %s: %w", nifcloud.ToString(zone.Name), err)
		}

		for _, record := range records.ResourceRecordSets {
			endpoints = append(endpoints, &endpoint.Endpoint{
				DNSName:       nifcloud.ToString(record.Name),
				Targets:       endpoint.NewTargets(resourceRecordValuesToStringSlice(record.ResourceRecords)...),
				RecordType:    nifcloud.ToString(record.Type),
				SetIdentifier: nifcloud.ToString(record.SetIdentifier),
				RecordTTL:     endpoint.TTL(nifcloud.ToInt32(record.TTL)),
			})
		}
	}

	return endpoints, nil
}

func (n *nifcloudProvider) ApplyChanges(ctx context.Context, changes *plan.Changes) error {
	log.Println("[DEBUG] ApplyChanges called")
	zones, err := n.client.ListHostedZones(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to fetch zone list: %w", err)
	}

	for _, c := range changes.Create {
		log.Printf("[DEBUG] ApplyChanges changes.Create: %#v", c)
		zone := getZoneOfRecord(zones.HostedZones, c.DNSName)
		if zone == "" {
			log.Printf("INFO: target zone for record %s is not found", c.DNSName)
			continue
		}

		req := &dns.ChangeResourceRecordSetsInput{
			ZoneID: nifcloud.String(zone),
			RequestChangeBatch: &types.RequestChangeBatch{
				ListOfRequestChanges: endpointToChanges(c, types.ActionOfChangeResourceRecordSetsRequestForChangeResourceRecordSetsCreate),
			},
		}

		if _, err := n.client.ChangeResourceRecordSets(ctx, req); err != nil {
			if strings.Contains(err.Error(), "REGISTERED RECORD") {
				continue
			}
			return fmt.Errorf("failed to create records (%#v): %w", c, err)
		}
	}

	for _, c := range changes.Delete {
		log.Printf("[DEBUG] ApplyChanges changes.Delete: %#v", c)
		zone := getZoneOfRecord(zones.HostedZones, c.DNSName)
		if zone == "" {
			log.Printf("INFO: target zone for record %s is not found", c.DNSName)
			continue
		}

		req := &dns.ChangeResourceRecordSetsInput{
			ZoneID: nifcloud.String(zone),
			RequestChangeBatch: &types.RequestChangeBatch{
				ListOfRequestChanges: endpointToChanges(c, types.ActionOfChangeResourceRecordSetsRequestForChangeResourceRecordSetsDelete),
			},
		}

		if _, err := n.client.ChangeResourceRecordSets(ctx, req); err != nil {
			if strings.Contains(err.Error(), "NO SUCH RECORD EXIST") {
				continue
			}
			return fmt.Errorf("failed to delete records (%#v): %w", c, err)
		}
	}

	return nil
}

func endpointToChanges(ep *endpoint.Endpoint, action types.ActionOfChangeResourceRecordSetsRequestForChangeResourceRecordSets) []types.RequestChanges {
	changes := make([]types.RequestChanges, len(ep.Targets))
	ttl := int32(ep.RecordTTL)
	if ttl == 0 {
		ttl = defaultTTL
	}

	for i, target := range ep.Targets {
		changes[i] = types.RequestChanges{
			RequestChange: &types.RequestChange{
				Action: action,
				RequestResourceRecordSet: &types.RequestResourceRecordSet{
					Name: nifcloud.String(ep.DNSName),
					Type: types.TypeOfChangeResourceRecordSetsRequestForChangeResourceRecordSets(ep.RecordType),
					TTL:  nifcloud.Int32(ttl),
					ListOfRequestResourceRecords: []types.RequestResourceRecords{
						{
							RequestResourceRecord: &types.RequestResourceRecord{
								Value: nifcloud.String(strings.Replace(target, "\"", "", -1)),
							},
						},
					},
				},
			},
		}
	}

	return changes
}

func getZoneOfRecord(zones []types.HostedZones, hostname string) string {
	for _, zone := range zones {
		if nifcloud.ToString(zone.Name) == hostname || strings.HasSuffix(hostname, "."+nifcloud.ToString(zone.Name)) {
			return nifcloud.ToString(zone.Name)
		}
	}
	return ""
}

func resourceRecordValuesToStringSlice(sets []types.ResourceRecords) []string {
	t := make([]string, len(sets))
	for i, s := range sets {
		// FIXME: NIFCLOUD DNS does not support `"`(double quote) in TXT content.
		if strings.HasPrefix(nifcloud.ToString(s.Value), "heritage") {
			t[i] = fmt.Sprintf(`"%s"`, nifcloud.ToString(s.Value))
		} else {
			t[i] = nifcloud.ToString(s.Value)
		}
	}
	return t
}
